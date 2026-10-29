# Go sample notes

Start with README.md, main.go and host.go. go.mod/go.sum pin the public SDK module; suite.lock pins the unchanged canonical lmcode-cpp corpus.

Root test.lms drives module and suite integrity checks, a CGO-disabled build, signature audits, 94 exact runtime cases, bytecode reproduction, runner controls and the annotated interface example. Install Go 1.26 and npm @lmctl-ai/lmctl. Both lmctl test.lms and lmscript test.lms work from this repository.

Actions validates Linux x64/arm64, macOS x64/arm64 and Windows x64 on pushes/PRs. Main publishes tested native binaries, licenses and SHA256SUMS after all jobs succeed. SAMPLE_VERSION sets the embedded --version value via Go linker flags. Python only maintains the canonical suite lock; Node adapts binary files/processes for lmscript tests.
