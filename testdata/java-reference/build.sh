#!/bin/bash
#
# Build script for the Java reference image generator
#
# This script compiles the standalone JpegEncoder and ReferenceGenerator
# for generating byte-compatibility reference images.
#
# Usage:
#   ./build.sh          - Compile all Java files
#   ./build.sh clean    - Remove compiled class files
#   ./build.sh run      - Compile and run the reference generator
#   ./build.sh help     - Show usage information

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check for Java
check_java() {
    if ! command -v javac &> /dev/null; then
        echo -e "${RED}Error: javac not found. Please install JDK.${NC}"
        echo "On macOS: brew install openjdk"
        echo "On Ubuntu: sudo apt install default-jdk"
        exit 1
    fi

    JAVA_VERSION=$(java -version 2>&1 | head -n 1 | cut -d'"' -f2 | cut -d'.' -f1)
    echo -e "${GREEN}Java version: $JAVA_VERSION${NC}"
}

# Compile all Java files
compile() {
    echo -e "${YELLOW}Compiling Java files...${NC}"
    javac -encoding UTF-8 *.java
    echo -e "${GREEN}Compilation successful.${NC}"
}

# Clean compiled files
clean() {
    echo -e "${YELLOW}Cleaning compiled files...${NC}"
    rm -f *.class
    echo -e "${GREEN}Clean complete.${NC}"
}

# Run the reference generator
run() {
    compile
    echo ""
    echo -e "${YELLOW}Running ReferenceGenerator...${NC}"
    echo ""

    OUTPUT_DIR="${1:-../reference}"
    java -Dfile.encoding=UTF-8 ReferenceGenerator "$OUTPUT_DIR"
}

# Show usage
show_help() {
    echo "Java Reference Image Generator Build Script"
    echo ""
    echo "Usage: ./build.sh [command] [options]"
    echo ""
    echo "Commands:"
    echo "  compile    Compile all Java files (default)"
    echo "  clean      Remove compiled class files"
    echo "  run [dir]  Compile and run the generator (output to dir, default: ../reference)"
    echo "  help       Show this help message"
    echo ""
    echo "Examples:"
    echo "  ./build.sh                    # Compile"
    echo "  ./build.sh run                # Generate to ../reference/"
    echo "  ./build.sh run /tmp/output    # Generate to /tmp/output/"
    echo "  ./build.sh clean              # Remove class files"
    echo ""
    echo "Java Files:"
    echo "  JpegEncoder.java      - James R. Weeks JPEG encoder (standalone)"
    echo "  JpegInfo.java         - Image metadata and YCbCr conversion"
    echo "  DCT.java              - DCT transform and quantization"
    echo "  Huffman.java          - Huffman entropy coding"
    echo "  ReferenceGenerator.java - Test pattern generation"
}

# Main
case "${1:-compile}" in
    compile)
        check_java
        compile
        ;;
    clean)
        clean
        ;;
    run)
        check_java
        run "$2"
        ;;
    help|--help|-h)
        show_help
        ;;
    *)
        echo -e "${RED}Unknown command: $1${NC}"
        show_help
        exit 1
        ;;
esac
