"""Prompt management module for CLI-LLM."""
import json
import os
from pathlib import Path
from typing import Dict, Any


# TODO: Remove the unsafe path join method
def get_shared_prompts_path() -> Path:
    """Get the path to the shared prompts directory."""
    # Get the project root directory (two levels up from this file)
    project_root = Path(__file__).parent.parent
    return project_root / "shared" / "prompts" / "system_prompts.json"

def load_prompts() -> Dict[str, Any]:
    """Load system prompts from the shared JSON file."""
    prompts_path = get_shared_prompts_path()
    if not prompts_path.exists():
        raise FileNotFoundError(f"Prompts file not found at {prompts_path}")
    
    with open(prompts_path, 'r', encoding='utf-8') as f:
        return json.load(f)

# Load prompts on module import
PROMPTS = load_prompts()

# Create model-specific mappings
SYS_ROLES = {
    'deepseek-coder': PROMPTS['coder']['content'],
    'deepseek-v3': PROMPTS['coder']['content'],
    'deepseek-chat': PROMPTS['chat']['content'],
    'deepseek-creative': PROMPTS['creative']['content']
}

MODEL_TEMPERATURES = {
    'deepseek-coder': PROMPTS['coder']['metadata']['temperature'],
    'deepseek-v3': PROMPTS['coder']['metadata']['temperature'],
    'deepseek-chat': PROMPTS['chat']['metadata']['temperature'],
    'deepseek-creative': PROMPTS['creative']['metadata']['temperature']
} 