#[allow(unused_imports)]
use substreams::log;

// Include generated protobuf bindings for Cardano types
mod pb {
    include!(concat!(env!("OUT_DIR"), "/sf.cardano.r#type.v1.rs"));
}

use pb::Block;

#[substreams::handlers::map]
fn map_blocks(block: Block) -> Result<Block, substreams::errors::Error> {
    if let Some(_header) = &block.header {
        log::info!("Processing Cardano block with header");
    } else {
        log::info!("Processing Cardano block without header");
    }
    Ok(block)
}
