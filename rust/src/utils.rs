use anyhow::Result;
use log::{LevelFilter, Metadata, Record};
use std::path::Path;

// Simple logging implementation - can be enhanced with fern or other logging crates later
pub fn init_logging() {
    // For now just setting a simple logger that prints to stdout
    // In a real implementation we would use a proper logging crate
    struct SimpleLogger;

    impl log::Log for SimpleLogger {
        fn enabled(&self, metadata: &Metadata) -> bool {
            metadata.level() <= log::Level::Info
        }

        fn log(&self, record: &Record) {
            if self.enabled(record.metadata()) {
                println!("{} - {}", record.level(), record.args());
            }
        }

        fn flush(&self) {}
    }

    // Use boxed logger approach with Box<dyn Log> as required by log crate 0.4.27
    let _ = log::set_boxed_logger(Box::new(SimpleLogger) as Box<dyn log::Log>)
        .map(|()| log::set_max_level(LevelFilter::Info));
}

// File operations utilities
pub fn ensure_parent_dir_exists(path: &str) -> Result<()> {
    if let Some(parent) = Path::new(path).parent() {
        if !parent.exists() {
            std::fs::create_dir_all(parent)?;
        }
    }
    Ok(())
}