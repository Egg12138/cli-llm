use std::env;
use anyhow::Result;

pub struct ApiConfig {
    pub api_key: String,
    pub api_endpoint: String,
}

pub enum ModelType {
    Coder,
    Chat,
    Creative,
    CoderReasoner,
    ChatReasoner,
    CreativeReasoner,
}

impl ModelType {
    pub fn from_str(s: &str) -> Result<Self> {
        match s {
            "coder" => Ok(Self::Coder),
            "chat" => Ok(Self::Chat),
            "creative" => Ok(Self::Creative),
            "coder-R" => Ok(Self::CoderReasoner),
            "chat-R" => Ok(Self::ChatReasoner),
            "creative-R" => Ok(Self::CreativeReasoner),
            _ => Err(anyhow::anyhow!("Unsupported model type: {}", s)),
        }
    }
}

pub fn get_api_config() -> Result<ApiConfig> {
    Ok(ApiConfig {
        api_key: env::var("OPENAI_API_KEY")?,
        api_endpoint: env::var("OPENAI_BASE_URL")?,
    })
}

// NOTICE:
// Proxy handling - simplified for now, can be expanded later
// Here I just set noproxy to localhost temporaily
pub fn configure_proxy() {
    // In a real implementation, this would set up proxy configuration
    // based on environment variables or system settings
    println!("Configuring proxy settings...");
}