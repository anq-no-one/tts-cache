// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "tts-cache-swift",
    platforms: [.iOS(.v16), .macOS(.v13)],
    products: [
        .library(name: "TTSCache", targets: ["TTSCache"]),
    ],
    targets: [
        .target(name: "TTSCache"),
        .testTarget(
            name: "TTSCacheTests",
            dependencies: ["TTSCache"]
        ),
    ]
)
