#!/usr/bin/env python3
"""
Simple LLM Agent for Agentainer
Supports multiple LLM providers: OpenAI GPT and Google Gemini
"""

import os
import json
import logging
from datetime import datetime
from flask import Flask, request, jsonify
import requests
from typing import Dict, Any, Optional

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = Flask(__name__)

# Agent configuration
AGENT_NAME = os.environ.get('AGENT_NAME', 'llm-agent')
LLM_PROVIDER = os.environ.get('LLM_PROVIDER', 'openai').lower()  # 'openai' or 'gemini'

# API Keys
OPENAI_API_KEY = os.environ.get('OPENAI_API_KEY', '')
GEMINI_API_KEY = os.environ.get('GEMINI_API_KEY', '')

# Model configuration
OPENAI_MODEL = os.environ.get('OPENAI_MODEL', 'gpt-3.5-turbo')
GEMINI_MODEL = os.environ.get('GEMINI_MODEL', 'gemini-pro')

# API Endpoints
OPENAI_API_URL = 'https://api.openai.com/v1/chat/completions'
GEMINI_API_URL = f'https://generativelanguage.googleapis.com/v1beta/models/{GEMINI_MODEL}:generateContent'

# State management
state_file = '/app/state/agent_state.json'
conversation_history = []


class LLMProvider:
    """Base class for LLM providers"""
    
    @staticmethod
    def get_provider(provider_name: str):
        """Factory method to get the appropriate provider"""
        if provider_name == 'openai':
            return OpenAIProvider()
        elif provider_name == 'gemini':
            return GeminiProvider()
        else:
            raise ValueError(f"Unsupported provider: {provider_name}")


class OpenAIProvider:
    """OpenAI GPT provider"""
    
    def __init__(self):
        if not OPENAI_API_KEY:
            raise ValueError("OPENAI_API_KEY environment variable is required for OpenAI provider")
        self.api_key = OPENAI_API_KEY
        self.model = OPENAI_MODEL
    
    def generate(self, prompt: str, system_prompt: str = None) -> Dict[str, Any]:
        """Generate response using OpenAI API"""
        messages = []
        
        if system_prompt:
            messages.append({"role": "system", "content": system_prompt})
        
        # Add conversation history
        for msg in conversation_history[-10:]:  # Keep last 10 messages for context
            messages.append(msg)
        
        messages.append({"role": "user", "content": prompt})
        
        headers = {
            'Authorization': f'Bearer {self.api_key}',
            'Content-Type': 'application/json'
        }
        
        data = {
            'model': self.model,
            'messages': messages,
            'temperature': 0.7,
            'max_tokens': 1000
        }
        
        try:
            response = requests.post(OPENAI_API_URL, headers=headers, json=data)
            response.raise_for_status()
            result = response.json()
            
            # Extract the response text
            response_text = result['choices'][0]['message']['content']
            
            # Update conversation history
            conversation_history.append({"role": "user", "content": prompt})
            conversation_history.append({"role": "assistant", "content": response_text})
            
            return {
                'response': response_text,
                'model': self.model,
                'provider': 'openai',
                'usage': result.get('usage', {})
            }
            
        except requests.exceptions.RequestException as e:
            logger.error(f"OpenAI API error: {str(e)}")
            raise Exception(f"Failed to generate response: {str(e)}")


class GeminiProvider:
    """Google Gemini provider"""
    
    def __init__(self):
        if not GEMINI_API_KEY:
            raise ValueError("GEMINI_API_KEY environment variable is required for Gemini provider")
        self.api_key = GEMINI_API_KEY
        self.model = GEMINI_MODEL
    
    def generate(self, prompt: str, system_prompt: str = None) -> Dict[str, Any]:
        """Generate response using Gemini API"""
        # Combine system prompt with user prompt for Gemini
        full_prompt = prompt
        if system_prompt:
            full_prompt = f"{system_prompt}\n\n{prompt}"
        
        # Add conversation context
        if conversation_history:
            context = "\n".join([
                f"{'User' if msg['role'] == 'user' else 'Assistant'}: {msg['content']}"
                for msg in conversation_history[-10:]
            ])
            full_prompt = f"Previous conversation:\n{context}\n\nUser: {full_prompt}"
        
        url = f"{GEMINI_API_URL}?key={self.api_key}"
        
        data = {
            'contents': [{
                'parts': [{
                    'text': full_prompt
                }]
            }],
            'generationConfig': {
                'temperature': 0.7,
                'maxOutputTokens': 1000
            }
        }
        
        try:
            response = requests.post(url, json=data)
            response.raise_for_status()
            result = response.json()
            
            # Extract the response text
            response_text = result['candidates'][0]['content']['parts'][0]['text']
            
            # Update conversation history
            conversation_history.append({"role": "user", "content": prompt})
            conversation_history.append({"role": "assistant", "content": response_text})
            
            return {
                'response': response_text,
                'model': self.model,
                'provider': 'gemini',
                'usage': {
                    'prompt_tokens': len(full_prompt.split()),
                    'completion_tokens': len(response_text.split())
                }
            }
            
        except requests.exceptions.RequestException as e:
            logger.error(f"Gemini API error: {str(e)}")
            raise Exception(f"Failed to generate response: {str(e)}")


def load_state():
    """Load agent state from persistent storage"""
    global conversation_history
    if os.path.exists(state_file):
        try:
            with open(state_file, 'r') as f:
                state = json.load(f)
                conversation_history = state.get('conversation_history', [])
                logger.info(f"Loaded state with {len(conversation_history)} messages")
        except Exception as e:
            logger.error(f"Failed to load state: {e}")


def save_state():
    """Save agent state to persistent storage"""
    try:
        os.makedirs(os.path.dirname(state_file), exist_ok=True)
        state = {
            'conversation_history': conversation_history[-50:],  # Keep last 50 messages
            'last_updated': datetime.now().isoformat()
        }
        with open(state_file, 'w') as f:
            json.dump(state, f, indent=2)
        logger.info("State saved successfully")
    except Exception as e:
        logger.error(f"Failed to save state: {e}")


@app.route('/health')
def health():
    """Health check endpoint"""
    try:
        # Check if we have valid API key for selected provider
        provider = LLMProvider.get_provider(LLM_PROVIDER)
        return jsonify({
            'status': 'healthy',
            'agent': AGENT_NAME,
            'provider': LLM_PROVIDER,
            'model': OPENAI_MODEL if LLM_PROVIDER == 'openai' else GEMINI_MODEL
        })
    except Exception as e:
        return jsonify({
            'status': 'unhealthy',
            'error': str(e)
        }), 503


@app.route('/status')
def status():
    """Get agent status and metrics"""
    return jsonify({
        'agent_name': AGENT_NAME,
        'provider': LLM_PROVIDER,
        'model': OPENAI_MODEL if LLM_PROVIDER == 'openai' else GEMINI_MODEL,
        'conversation_length': len(conversation_history),
        'uptime': datetime.now().isoformat()
    })


@app.route('/chat', methods=['POST'])
def chat():
    """Main chat endpoint"""
    try:
        data = request.get_json()
        if not data or 'message' not in data:
            return jsonify({'error': 'Missing message in request body'}), 400
        
        message = data['message']
        system_prompt = data.get('system_prompt', 'You are a helpful AI assistant.')
        
        # Get the appropriate provider
        provider = LLMProvider.get_provider(LLM_PROVIDER)
        
        # Generate response
        result = provider.generate(message, system_prompt)
        
        # Save state after each conversation
        save_state()
        
        return jsonify({
            'success': True,
            'data': result,
            'timestamp': datetime.now().isoformat()
        })
        
    except Exception as e:
        logger.error(f"Chat error: {str(e)}")
        return jsonify({
            'success': False,
            'error': str(e)
        }), 500


@app.route('/clear', methods=['POST'])
def clear_history():
    """Clear conversation history"""
    global conversation_history
    conversation_history = []
    save_state()
    
    return jsonify({
        'success': True,
        'message': 'Conversation history cleared'
    })


@app.route('/history', methods=['GET'])
def get_history():
    """Get conversation history"""
    return jsonify({
        'history': conversation_history,
        'count': len(conversation_history)
    })


@app.route('/')
def index():
    """Root endpoint with usage information"""
    return jsonify({
        'agent': AGENT_NAME,
        'type': 'LLM Agent',
        'provider': LLM_PROVIDER,
        'endpoints': {
            '/': 'This help message',
            '/health': 'Health check',
            '/status': 'Agent status and metrics',
            '/chat': 'Send chat message (POST)',
            '/clear': 'Clear conversation history (POST)',
            '/history': 'Get conversation history (GET)'
        },
        'usage': {
            'chat': {
                'method': 'POST',
                'body': {
                    'message': 'Your message here',
                    'system_prompt': 'Optional system prompt'
                }
            }
        }
    })


if __name__ == '__main__':
    # Load state on startup
    load_state()
    
    # Start the Flask app
    port = int(os.environ.get('PORT', 8000))
    app.run(host='0.0.0.0', port=port)