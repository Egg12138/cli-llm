use anyhow::Result;
use std::io::{self, Read};

pub fn read_stdin_input() -> Result<String> {
    let mut stdin_input = String::new();
    if io::stdin().read_to_string(&mut stdin_input)? > 0 {
        Ok(stdin_input.trim().to_string())
    } else {
        Ok(String::new())
    }
}

pub fn get_effective_prompt(prompt: Option<String>, prompt_opt: Option<String>, stdin_input: &str) -> Result<String> {
    match (prompt, prompt_opt, !stdin_input.is_empty()) {
        (Some(p), _, _) => Ok(p),
        (None, Some(p), _) => Ok(p),
        (None, None, true) => Ok(stdin_input.to_string()),
        _ => Err(anyhow::anyhow!("No prompt provided. Please provide input through command line argument or stdin.")),
    }
}