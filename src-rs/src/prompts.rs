use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::fs;
use std::path::Path;

#[derive(Debug, Serialize, Deserialize)]
pub struct PromptContent {
    pub ROLE: Option<String>,
    pub Commands: Option<Vec<serde_json::Value>>,
    #[serde(rename = "Description of function command")]
    pub description: Option<String>,
    #[serde(rename = "Additional TIPS")]
    pub tips: Option<String>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct PromptMetadata {
    pub description: String,
    pub temperature: f32,
    pub model_name: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct Prompt {
    pub content: PromptContent,
    pub metadata: PromptMetadata,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct Prompts {
    pub coder: Prompt,
    pub chat: Prompt,
    pub creative: Prompt,
}

pub struct PromptManager {
    prompts: Prompts,
    sys_roles: HashMap<String, String>,
    temperatures: HashMap<String, f32>,
}

impl PromptManager {
    pub fn new() -> Result<Self, Box<dyn std::error::Error>> {
        let project_root = Path::new(env!("CARGO_MANIFEST_DIR")).parent().unwrap().parent().unwrap();
        let prompts_path = project_root.join("shared").join("prompts").join("system_prompts.json");
        
        let prompts_str = fs::read_to_string(prompts_path)?;
        let prompts: Prompts = serde_json::from_str(&prompts_str)?;
        
        let mut sys_roles = HashMap::new();
        let mut temperatures = HashMap::new();
        
        // Map coder prompts
        sys_roles.insert("deepseek-coder".to_string(), serde_json::to_string(&prompts.coder.content)?);
        sys_roles.insert("deepseek-v3".to_string(), serde_json::to_string(&prompts.coder.content)?);
        temperatures.insert("deepseek-coder".to_string(), prompts.coder.metadata.temperature);
        temperatures.insert("deepseek-v3".to_string(), prompts.coder.metadata.temperature);
        
        // Map chat prompts
        sys_roles.insert("deepseek-chat".to_string(), serde_json::to_string(&prompts.chat.content)?);
        temperatures.insert("deepseek-chat".to_string(), prompts.chat.metadata.temperature);
        
        // Map creative prompts
        sys_roles.insert("deepseek-creative".to_string(), serde_json::to_string(&prompts.creative.content)?);
        temperatures.insert("deepseek-creative".to_string(), prompts.creative.metadata.temperature);
        
        Ok(Self {
            prompts,
            sys_roles,
            temperatures,
        })
    }
    
    pub fn get_sys_role(&self, model: &str) -> Option<&str> {
        self.sys_roles.get(model).map(|s| s.as_str())
    }
    
    pub fn get_temperature(&self, model: &str) -> Option<f32> {
        self.temperatures.get(model).copied()
    }
}