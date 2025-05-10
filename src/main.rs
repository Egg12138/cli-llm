mod cli;
mod config;
mod llm;
mod utils;

use anyhow::Result;
use clap::Parser;
use config::ModelType;
use utils::init_logging;
use llm::{LlmService, api};

#[derive(Parser, Debug)]
#[command(author, version, about, long_about = None)]
struct Args {
    #[arg(help = "Input prompt for deepseek")]
    prompt: Option<String>,
    
    #[arg(short = 'p', help = "Input prompt for deepseek")]
    prompt_opt: Option<String>,
    
    #[arg(short = 'n', long = "no-stream", help = "Disable streaming of the response")]
    no_stream: bool,
    
    #[arg(short = 'm', long = "model", default_value = "coder", help = "Choose the model to use")]
    model: String,
    
    #[arg(short = 'o', long = "output-codes", help = "Output the code to the target file")]
    output_codes: Option<String>,
    
    #[arg(short = 'd', long = "debug", help = "Enable debug mode to show detailed logs")]
    debug: bool,
}

fn main() -> Result<()> {
    // Initialize logging
    init_logging();
    
    // Parse command line arguments
    let args = Args::parse();
    // Get input from stdin if available
    let stdin_input = cli::read_stdin_input()?;
    
    // Get prompt from various sources
    let prompt = cli::get_effective_prompt(args.prompt, args.prompt_opt, &stdin_input)?;
    
    // Configure proxy settings
    config::configure_proxy();
    
    // Get API configuration
    let api_config = config::get_api_config()?;
    
    // Convert model string to ModelType enum
    let model_type = ModelType::from_str(&args.model)?;
    
    // Create LLM service instance
    let llm_service = LlmService::new(api_config.api_key, api_config.api_endpoint);
    
    // Process based on model type
    if args.no_stream {
        // Non-streaming response
        println!("Using non-streaming mode");
        let response = llm_service.send_request(&model_type, &prompt, false)?;
        println!("Response: {}", response);
    } else {
        // Streaming response
        println!("Using streaming mode");
        api::stream_response(&model_type, &prompt, |chunk| {
            print!("{}", chunk);
        })?;
        println!(); // Newline at the end of streaming
    }
    
    Ok(())
}
