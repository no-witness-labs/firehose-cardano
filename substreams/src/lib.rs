#[allow(unused_imports)]
use substreams::log;

// Include generated protobuf bindings for Cardano types
mod pb {
    include!(concat!(env!("OUT_DIR"), "/sf.cardano.r#type.v1.rs"));
}

use pb::Block;

#[substreams::handlers::map]
fn map_blocks(block: Block) -> Result<Block, substreams::errors::Error> {
    // Extract meaningful data from the Cardano block
    if let Some(header) = &block.header {
        log::info!("Processing Cardano block - Slot: {}, Hash: {}", 
            header.slot, 
            hex::encode(&header.hash)
        );
        
        // Log block size and transaction count
        if let Some(body) = &block.body {
            log::info!("Block contains {} transactions", body.tx.len());
            
            // Process each transaction
            for (i, tx) in body.tx.iter().enumerate() {
                log::info!("Transaction {}: {} inputs, {} outputs, fee: {} lovelace", 
                    i,
                    tx.inputs.len(),
                    tx.outputs.len(),
                    tx.fee
                );
                
                // Process outputs with ADA amounts
                for (j, output) in tx.outputs.iter().enumerate() {
                    log::info!("  Output {}: {} lovelace", 
                        j,
                        output.coin
                    );
                }
                
                // Process minted/burned assets
                for mint_group in &tx.mint {
                    log::info!("  Mint/Burn Policy: {}", hex::encode(&mint_group.policy_id));
                    for asset in &mint_group.assets {
                        // mint_coin is i64 and can be negative (burn) or positive (mint)
                        if asset.mint_coin > 0 {
                            log::info!("    Minted: {} = +{}", 
                                hex::encode(&asset.name),
                                asset.mint_coin
                            );
                        } else if asset.mint_coin < 0 {
                            log::info!("    Burned: {} = {}", 
                                hex::encode(&asset.name),
                                asset.mint_coin
                            );
                        }
                    }
                }
            }
        }
    } else {
        log::info!("Block missing header information");
    }
    
    // Log timestamp
    log::info!("Block timestamp: {}", block.timestamp);
    
    Ok(block)
}
