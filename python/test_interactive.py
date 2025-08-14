#!/usr/bin/env python3
"""Test script for interactive mode."""
import sys
import os

# Add the parent directory to sys.path
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from cli_llm.interactive import InteractiveSession

def test_interactive_session():
    """Test the interactive session."""
    print("=== Interactive Session Test ===")
    
    # Create session
    session = InteractiveSession()
    print(f"Created session with role: {session.current_role}")
    
    # Test status display
    print("\n--- Status Display ---")
    session.display_status()
    
    # Test command processing
    print("\n--- Command Processing ---")
    test_commands = [
        "/help",
        "/role chat",
        "/role invalid",
        "/role",
        "/unknown"
    ]
    
    for cmd in test_commands:
        print(f"\nTesting command: {cmd}")
        is_cmd, cmd_name, args = session.command_processor.parse_command(cmd)
        if is_cmd:
            result = session.command_processor.execute_command(cmd_name, args, session)
            if result:
                print(result)
        else:
            print("Not a command")
    
    # Test reference handling
    print("\n--- Reference Handling ---")
    test_references = [
        "@This is a test reference",
        "@/etc/hosts"  # This should try to read a file
    ]
    
    for ref in test_references:
        print(f"\nTesting reference: {ref}")
        result = session.handle_reference(ref)
        if result:
            print(result)
    
    print("\n=== Test Complete ===")

if __name__ == "__main__":
    test_interactive_session()