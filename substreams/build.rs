use std::env;
use std::path::PathBuf;

fn main() {
    let out_dir = PathBuf::from(env::var("OUT_DIR").unwrap());
    
    // Generate Rust code from our Cardano protobuf files
    prost_build::Config::new()
        .out_dir(&out_dir)
        .compile_protos(&["../proto/sf/cardano/type/v1/type.proto"], &["../proto"])
        .unwrap();
        
    println!("cargo:rerun-if-changed=../proto/sf/cardano/type/v1/type.proto");
}
