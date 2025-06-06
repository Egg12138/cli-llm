"""Model configuration management for CLI-LLM."""
from dataclasses import dataclass
from typing import Dict, Optional, Tuple
from .prompts import PROMPTS

@dataclass
class ModelConfig:
    """Configuration for a model."""
    model_name: str
    temperature: float
    sys_role_key: str

class ModelManager:
    """Manages model configurations and mappings."""
    
    # Predefined model shortcuts
    SHORTCUTS = {
        'coder': ModelConfig('deepseek-coder', 0.0, 'coder'),
        'coder-R': ModelConfig('deepseek-reasoner', 0.0, 'coder'),
        'chat': ModelConfig('deepseek-chat', 1.3, 'chat'),
        'chat-R': ModelConfig('deepseek-reasoner', 1.3, 'chat'),
        'creative': ModelConfig('deepseek-chat', 1.5, 'creative'),
        'creative-R': ModelConfig('deepseek-reasoner', 1.5, 'creative'),
    }

    @classmethod
    def get_model_config(cls, model_input: str) -> Tuple[str, float, str]:
        """
        Get model configuration from input.
        
        Args:
            model_input: Model name or shortcut
            
        Returns:
            Tuple of (model_name, temperature, system_role)
            For custom models, uses the input directly as model name with default settings
        """
        # Check if it's a predefined shortcut
        if model_input in cls.SHORTCUTS:
            config = cls.SHORTCUTS[model_input]
            return config.model_name, config.temperature, PROMPTS[config.sys_role_key]['content']
        
        # For custom models, use defaults with chat prompt
        return model_input, 1.0, PROMPTS['chat']['content']

    @classmethod
    def is_valid_shortcut(cls, model_input: str) -> bool:
        """Check if the input is a valid predefined shortcut."""
        return model_input in cls.SHORTCUTS

    @classmethod
    def get_available_shortcuts(cls) -> list[str]:
        """Get list of available model shortcuts."""
        return list(cls.SHORTCUTS.keys()) 