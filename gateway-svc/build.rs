// Compiles the service-mgt registry proto into Rust client stubs (tonic/prost).
// Requires `protoc` at build time (installed in the Docker builder stage).
fn main() -> Result<(), Box<dyn std::error::Error>> {
    tonic_build::configure()
        .build_server(false)
        .compile_protos(&["proto/registry.proto"], &["proto"])?;
    Ok(())
}
