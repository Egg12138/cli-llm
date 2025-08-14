"""Interactive mode for CLI LLM."""
import os
import sys
from typing import Optional, Tuple, Dict, Callable

# Add the parent directory to sys.path to import from the main module
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from prompts import SYS_ROLES, SystemPrompt


class CommandProcessor:
    """Process slash commands."""
    
    def __init__(self):
        self.commands = {
            'help': self._handle_help,
            'quit': self._handle_quit,
            'role': self._handle_role,
        }
    
    def parse_command(self, input_text: str) -> Tuple[bool, Optional[str], Optional[str]]:
        """
        Parse user input to identify if it's a command.
        Returns: (is_command, command_name, args)
        """
        input_text = input_text.strip()
        
        # Check if it starts with /
        if not input_text.startswith('/'):
            return False, None, None
            
        # Split command and arguments
        parts = input_text[1:].split(' ', 1)
        command_name = parts[0].lower() if parts else ''
        args = parts[1] if len(parts) > 1 else ''
        
        return True, command_name, args
    
    def execute_command(self, command_name: str, args: str, session: 'InteractiveSession') -> Optional[str]:
        """Execute a command."""
        if command_name in self.commands:
            return self.commands[command_name](args, session)
        else:
            return self._handle_unknown_command(command_name)
    
    def _handle_help(self, args: str, session: 'InteractiveSession') -> str:
        """Handle /help command."""
        help_text = """Available commands:
  /help              - Show this help message
  /quit              - Exit the program
  /role [role_name]  - Switch system role (coder, chat, creative, smart)
  /role              - Show current role and available roles
"""
        return help_text
    
    def _handle_quit(self, args: str, session: 'InteractiveSession') -> str:
        """Handle /quit command."""
        print("Goodbye!")
        sys.exit(0)
    
    def _handle_role(self, args: str, session: 'InteractiveSession') -> str:
        """Handle /role command."""
        args = args.strip()
        
        # If no arguments, show current role and available roles
        if not args:
            available_roles = ", ".join(session.sys_roles.keys())
            return f"Current role: {session.current_role}\nAvailable roles: {available_roles}"
        
        # If arguments provided, try to switch role
        if args in session.sys_roles:
            old_role = session.current_role
            session.current_role = args
            return f"Switched role from '{old_role}' to '{args}'"
        else:
            available_roles = ", ".join(session.sys_roles.keys())
            return f"Unknown role: {args}\nAvailable roles: {available_roles}"
    
    def _handle_unknown_command(self, command_name: str) -> str:
        """Handle unknown command."""
        return f"Unknown command: /{command_name}\nType /help for available commands."


class InteractiveSession:
    """Interactive session for CLI LLM."""
    
    def __init__(self):
        self.current_role = "coder"
        self.model = os.getenv("OPENAI_MODEL", "default-model")
        self.base_url = os.getenv("OPENAI_BASE_URL", "default-url")
        self.temperature = 1.0
        self.referenced_text = ""
        self.sys_roles = SYS_ROLES
        self.command_processor = CommandProcessor()
        
    def get_current_path(self) -> str:
        """Get current working directory in a shortened format."""
        cwd = os.getcwd()
        home = os.path.expanduser("~")
        if cwd.startswith(home):
            return "~" + cwd[len(home):]
        return cwd
    
    def display_status(self) -> None:
        """Display status line similar to qwen-code footer."""
        path = self.get_current_path()
        model = self.model
        endpoint = self.base_url
        
        print(f"[{path}] model: {model} endpoint: {endpoint}")
    
    def display_prompt(self) -> None:
        """Display input prompt."""
        print(">>> ", end="", flush=True)
    
    def handle_reference(self, text: str) -> Optional[str]:
        """Handle @ reference text."""
        if text.startswith('@'):
            # Remove @ and strip whitespace
            ref_text = text[1:].strip()
            
            # If it's a file path, try to read the file
            if os.path.exists(ref_text):
                try:
                    with open(ref_text, 'r', encoding='utf-8') as f:
                        content = f.read()
                    self.referenced_text = content
                    return f"Referenced file: {ref_text} ({len(content)} chars)"
                except Exception as e:
                    return f"Error reading file: {e}"
            else:
                # Treat as direct text reference
                self.referenced_text = ref_text
                return f"Referenced text: {ref_text[:50]}{'...' if len(ref_text) > 50 else ''}"
        return None
    
    def process_input(self, user_input: str) -> Optional[str]:
        """Process user input."""
        user_input = user_input.strip()
        
        # Handle empty input
        if not user_input:
            return None
            
        # Handle reference text (@ prefix)
        if user_input.startswith('@'):
            result = self.handle_reference(user_input)
            if result:
                print(result)
            return None
            
        # Check if it's a command
        is_command, command_name, args = self.command_processor.parse_command(user_input)
        
        if is_command:
            # Execute command
            result = self.command_processor.execute_command(command_name, args, self)
            if result:
                print(result)
            return None
        
        # Regular query, combine reference text and user input
        full_prompt = user_input
        if self.referenced_text:
            full_prompt = f"{self.referenced_text}\n\n{user_input}"
            # Clear used reference text
            self.referenced_text = ""
            
        return full_prompt
    
    def call_llm(self, prompt: str) -> None:
        """Call LLM with the prompt."""
        # This will integrate with the existing LLM logic
        from ..llm import chat
        import openai
        
        # Handle empty API key - some local LLM servers don't require it
        from ..llm import resolve_key
        API_KEY = os.getenv("OPENAI_API_KEY")
        api_key = resolve_key(API_KEY)
        client = openai.OpenAI(api_key=api_key, base_url=self.base_url)
        
        # Get system role
        sys_role = self.sys_roles[self.current_role]
        
        # Call existing chat function
        try:
            chat(
                prompt=prompt,
                no_stream=False,  # Enable streaming for interactive mode
                model=self.model,
                sys_role=sys_role,
                client=client,
                count_tokens=False,  # Could be enabled as an option
                custom_temp=None  # Could use self.temperature
            )
        except Exception as e:
            print(f"Error calling LLM: {e}")
    
    def run(self) -> None:
        """Run the interactive session."""
        print("LLM Interactive Mode")
        print("Type /help for available commands")
        print()
        
        while True:
            try:
                # Display status and prompt
                self.display_status()
                self.display_prompt()
                
                # Get user input
                user_input = input()
                
                # Process user input
                prompt = self.process_input(user_input)
                
                # If we have a prompt to process, call LLM
                if prompt:
                    self.call_llm(prompt)
                    print()  # Add blank line after response
                    
            except KeyboardInterrupt:
                print("\nUse /quit to exit")
            except EOFError:
                print("\nGoodbye!")
                break
