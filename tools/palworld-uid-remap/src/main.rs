use std::path::PathBuf;
use std::process::ExitCode;

use clap::Parser;
use palworld_uid_remap::{remap_world, MappingSet, RemapOptions};

#[derive(Debug, Parser)]
#[command(name = "palworld-uid-remap")]
struct Args {
    #[arg(long, value_name = "WORLD_DIR")]
    input: PathBuf,
    #[arg(long, value_name = "OUTPUT_DIR")]
    output: PathBuf,
    #[arg(long, value_name = "PRIVATE_JSON")]
    mapping: PathBuf,
}

fn main() -> ExitCode {
    match run(Args::parse()) {
        Ok(()) => ExitCode::SUCCESS,
        Err(message) => {
            eprintln!("palworld-uid-remap: {message}");
            ExitCode::FAILURE
        }
    }
}

fn run(args: Args) -> Result<(), String> {
    let mapping_bytes = std::fs::read(&args.mapping)
        .map_err(|error| format!("could not read mapping file: {error}"))?;
    let mapping = MappingSet::from_json(&mapping_bytes).map_err(|error| error.to_string())?;
    let report = remap_world(&args.input, &args.output, &mapping, &RemapOptions::default())
        .map_err(|error| error.to_string())?;
    serde_json::to_writer_pretty(std::io::stdout().lock(), &report)
        .map_err(|error| format!("could not serialize verification report: {error}"))?;
    println!();
    Ok(())
}
