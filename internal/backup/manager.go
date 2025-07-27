package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agentainer/agentainer-lab/internal/agent"
	"github.com/go-redis/redis/v8"
)

// Backup represents a backup of agent configurations and data
type Backup struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	CreatedAt   time.Time         `json:"created_at"`
	Agents      []BackupAgent     `json:"agents"`
	Version     string            `json:"version"`
}

// BackupAgent represents an agent in the backup
type BackupAgent struct {
	Agent       *agent.Agent      `json:"agent"`
	VolumeData  map[string]string `json:"volume_data"` // path -> base64 encoded tar.gz
}

// Manager handles backup and restore operations
type Manager struct {
	agentMgr    *agent.Manager
	redisClient *redis.Client
	backupDir   string
}

// NewManager creates a new backup manager
func NewManager(agentMgr *agent.Manager, redisClient *redis.Client, backupDir string) *Manager {
	// Default backup directory
	if backupDir == "" {
		homeDir, _ := os.UserHomeDir()
		backupDir = filepath.Join(homeDir, ".agentainer", "backups")
	}
	
	// Create backup directory if it doesn't exist
	os.MkdirAll(backupDir, 0755)
	
	return &Manager{
		agentMgr:    agentMgr,
		redisClient: redisClient,
		backupDir:   backupDir,
	}
}

// CreateBackup creates a backup of specified agents (or all if empty)
func (m *Manager) CreateBackup(ctx context.Context, name, description string, agentIDs []string) (*Backup, error) {
	backup := &Backup{
		ID:          fmt.Sprintf("backup-%d", time.Now().Unix()),
		Name:        name,
		Description: description,
		CreatedAt:   time.Now(),
		Version:     "1.0",
		Agents:      []BackupAgent{},
	}
	
	// Get agents to backup
	var agentsToBackup []agent.Agent
	if len(agentIDs) == 0 {
		// Backup all agents
		agents, err := m.agentMgr.ListAgents("")
		if err != nil {
			return nil, fmt.Errorf("failed to list agents: %w", err)
		}
		agentsToBackup = agents
	} else {
		// Backup specific agents
		for _, id := range agentIDs {
			a, err := m.agentMgr.GetAgent(id)
			if err != nil {
				log.Printf("Warning: Failed to get agent %s: %v", id, err)
				continue
			}
			agentsToBackup = append(agentsToBackup, *a)
		}
	}
	
	// Backup each agent
	for _, a := range agentsToBackup {
		agentCopy := a // Make a copy
		backupAgent := BackupAgent{
			Agent:      &agentCopy,
			VolumeData: make(map[string]string),
		}
		
		// Backup volume data if agent has volumes
		if len(a.Volumes) > 0 {
			for _, vol := range a.Volumes {
				data, err := m.backupVolume(vol.HostPath)
				if err != nil {
					log.Printf("Warning: Failed to backup volume %s: %v", vol.HostPath, err)
					continue
				}
				backupAgent.VolumeData[vol.HostPath] = data
			}
		}
		
		backup.Agents = append(backup.Agents, backupAgent)
	}
	
	// Save backup to file
	backupFile := filepath.Join(m.backupDir, backup.ID+".json")
	data, err := json.MarshalIndent(backup, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal backup: %w", err)
	}
	
	if err := os.WriteFile(backupFile, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write backup file: %w", err)
	}
	
	log.Printf("Backup created: %s (%d agents)", backup.ID, len(backup.Agents))
	
	return backup, nil
}

// RestoreBackup restores agents from a backup
func (m *Manager) RestoreBackup(ctx context.Context, backupID string, agentIDs []string) error {
	// Load backup
	backup, err := m.LoadBackup(backupID)
	if err != nil {
		return fmt.Errorf("failed to load backup: %w", err)
	}
	
	// Filter agents to restore
	agentsToRestore := backup.Agents
	if len(agentIDs) > 0 {
		filtered := []BackupAgent{}
		for _, ba := range backup.Agents {
			for _, id := range agentIDs {
				if ba.Agent.ID == id || ba.Agent.Name == id {
					filtered = append(filtered, ba)
					break
				}
			}
		}
		agentsToRestore = filtered
	}
	
	// Restore each agent
	restoredCount := 0
	for _, ba := range agentsToRestore {
		// Restore volume data first
		for path, data := range ba.VolumeData {
			if err := m.restoreVolume(path, data); err != nil {
				log.Printf("Warning: Failed to restore volume %s: %v", path, err)
			}
		}
		
		// Deploy the agent
		_, err := m.agentMgr.Deploy(
			ctx,
			ba.Agent.Name+"-restored",
			ba.Agent.Image,
			ba.Agent.EnvVars,
			ba.Agent.CPULimit,
			ba.Agent.MemoryLimit,
			ba.Agent.AutoRestart,
			ba.Agent.Token,
			ba.Agent.Ports,
			ba.Agent.Volumes,
			ba.Agent.HealthCheck,
		)
		
		if err != nil {
			log.Printf("Failed to restore agent %s: %v", ba.Agent.Name, err)
			continue
		}
		
		restoredCount++
	}
	
	log.Printf("Restored %d/%d agents from backup %s", restoredCount, len(agentsToRestore), backupID)
	
	return nil
}

// ListBackups returns all available backups
func (m *Manager) ListBackups() ([]*Backup, error) {
	files, err := os.ReadDir(m.backupDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read backup directory: %w", err)
	}
	
	backups := []*Backup{}
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		
		backup, err := m.LoadBackup(strings.TrimSuffix(file.Name(), ".json"))
		if err != nil {
			log.Printf("Warning: Failed to load backup %s: %v", file.Name(), err)
			continue
		}
		
		backups = append(backups, backup)
	}
	
	return backups, nil
}

// LoadBackup loads a backup from file
func (m *Manager) LoadBackup(backupID string) (*Backup, error) {
	backupFile := filepath.Join(m.backupDir, backupID+".json")
	data, err := os.ReadFile(backupFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read backup file: %w", err)
	}
	
	var backup Backup
	if err := json.Unmarshal(data, &backup); err != nil {
		return nil, fmt.Errorf("failed to unmarshal backup: %w", err)
	}
	
	return &backup, nil
}

// DeleteBackup deletes a backup
func (m *Manager) DeleteBackup(backupID string) error {
	backupFile := filepath.Join(m.backupDir, backupID+".json")
	return os.Remove(backupFile)
}

// backupVolume creates a tar.gz of a directory and returns base64 encoded data
func (m *Manager) backupVolume(path string) (string, error) {
	// Skip if path doesn't exist
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", nil
	}
	
	// Create temporary file for tar.gz
	tmpFile, err := os.CreateTemp("", "volume-*.tar.gz")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()
	
	// Create gzip writer
	gw := gzip.NewWriter(tmpFile)
	defer gw.Close()
	
	// Create tar writer
	tw := tar.NewWriter(gw)
	defer tw.Close()
	
	// Walk directory and add files to tar
	err = filepath.Walk(path, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		// Create tar header
		header, err := tar.FileInfoHeader(fi, fi.Name())
		if err != nil {
			return err
		}
		
		// Update header name to be relative to base path
		relPath, err := filepath.Rel(path, file)
		if err != nil {
			return err
		}
		header.Name = relPath
		
		// Write header
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		
		// Write file content if not a directory
		if !fi.IsDir() {
			data, err := os.Open(file)
			if err != nil {
				return err
			}
			defer data.Close()
			
			if _, err := io.Copy(tw, data); err != nil {
				return err
			}
		}
		
		return nil
	})
	
	if err != nil {
		return "", fmt.Errorf("failed to create tar: %w", err)
	}
	
	// Close writers to flush data
	tw.Close()
	gw.Close()
	tmpFile.Close()
	
	// Read and encode file
	data, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return "", fmt.Errorf("failed to read tar file: %w", err)
	}
	
	// For simplicity, we'll store the path to the temp file instead of base64
	// In production, you'd want to use proper storage
	backupPath := filepath.Join(m.backupDir, "volumes", fmt.Sprintf("%d.tar.gz", time.Now().UnixNano()))
	os.MkdirAll(filepath.Dir(backupPath), 0755)
	
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to save volume backup: %w", err)
	}
	
	return backupPath, nil
}

// restoreVolume restores a volume from backup
func (m *Manager) restoreVolume(path, backupPath string) error {
	if backupPath == "" {
		return nil
	}
	
	// Read backup file
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup file: %w", err)
	}
	
	// Create gzip reader
	gr, err := gzip.NewReader(strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gr.Close()
	
	// Create tar reader
	tr := tar.NewReader(gr)
	
	// Extract files
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}
		
		// Construct full path
		target := filepath.Join(path, header.Name)
		
		// Create directory if needed
		if header.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
			continue
		}
		
		// Create parent directory
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return fmt.Errorf("failed to create parent directory: %w", err)
		}
		
		// Create file
		file, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.FileMode(header.Mode))
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}
		
		// Copy file content
		if _, err := io.Copy(file, tr); err != nil {
			file.Close()
			return fmt.Errorf("failed to extract file: %w", err)
		}
		
		file.Close()
	}
	
	return nil
}

// ExportBackup exports a backup as a tar.gz file
func (m *Manager) ExportBackup(backupID, outputPath string) error {
	backup, err := m.LoadBackup(backupID)
	if err != nil {
		return fmt.Errorf("failed to load backup: %w", err)
	}
	
	// Create output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()
	
	// Create gzip writer
	gw := gzip.NewWriter(outFile)
	defer gw.Close()
	
	// Create tar writer
	tw := tar.NewWriter(gw)
	defer tw.Close()
	
	// Add backup metadata
	metadataJSON, _ := json.MarshalIndent(backup, "", "  ")
	header := &tar.Header{
		Name: "backup.json",
		Size: int64(len(metadataJSON)),
		Mode: 0644,
	}
	tw.WriteHeader(header)
	tw.Write(metadataJSON)
	
	// Add volume backups
	for _, ba := range backup.Agents {
		for path, backupPath := range ba.VolumeData {
			if backupPath == "" {
				continue
			}
			
			// Read volume backup
			data, err := os.ReadFile(backupPath)
			if err != nil {
				log.Printf("Warning: Failed to read volume backup %s: %v", backupPath, err)
				continue
			}
			
			// Add to tar
			header := &tar.Header{
				Name: fmt.Sprintf("volumes/%s-%s.tar.gz", ba.Agent.Name, filepath.Base(path)),
				Size: int64(len(data)),
				Mode: 0644,
			}
			tw.WriteHeader(header)
			tw.Write(data)
		}
	}
	
	log.Printf("Exported backup %s to %s", backupID, outputPath)
	
	return nil
}