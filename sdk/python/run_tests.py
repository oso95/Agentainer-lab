#!/usr/bin/env python3
"""
Run all SDK tests
"""

import sys
import pytest

def main():
    """Run the test suite"""
    args = [
        "-v",  # Verbose output
        "--tb=short",  # Short traceback format
        "--color=yes",  # Colored output
        "tests/",  # Test directory
    ]
    
    # Add any additional arguments passed to the script
    args.extend(sys.argv[1:])
    
    # Run pytest
    exit_code = pytest.main(args)
    
    return exit_code


if __name__ == "__main__":
    sys.exit(main())
