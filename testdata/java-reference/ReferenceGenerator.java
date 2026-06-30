// ReferenceGenerator - Generates synthetic test patterns for byte-compatibility testing
// Part of the f5encoder byte-compatibility verification suite
//
// This tool generates deterministic test patterns and encodes them using the
// James R. Weeks JpegEncoder, creating reference images for comparison with
// the Go f5encoder implementation.

import java.awt.Color;
import java.awt.image.BufferedImage;
import java.io.*;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;

/**
 * ReferenceGenerator creates synthetic test images and encodes them
 * using the James R. Weeks JpegEncoder for byte-compatibility testing.
 */
public class ReferenceGenerator {

    // Test pattern types
    public static final int PATTERN_SOLID = 0;
    public static final int PATTERN_HORIZONTAL_GRADIENT = 1;
    public static final int PATTERN_VERTICAL_GRADIENT = 2;
    public static final int PATTERN_DIAGONAL_GRADIENT = 3;
    public static final int PATTERN_CHECKERBOARD = 4;
    public static final int PATTERN_QUADRANT = 5;

    // Pattern names for output
    private static final String[] PATTERN_NAMES = {
        "solid",
        "horizontal_gradient",
        "vertical_gradient",
        "diagonal_gradient",
        "checkerboard",
        "quadrant"
    };

    // Quality levels to test
    private static final int[] QUALITY_LEVELS = { 1, 10, 25, 50, 75, 90, 95, 100 };

    // Test dimensions
    private static final int[][] DIMENSIONS = {
        { 8, 8 },      // Single MCU
        { 64, 64 },    // Standard
        { 256, 256 },  // Comprehensive
        { 33, 33 },    // Non-multiple of 8 (square)
        { 100, 75 }    // Non-multiple of 8 (rectangular)
    };

    /**
     * Main entry point.
     * Usage: java ReferenceGenerator [output_dir]
     */
    public static void main(String[] args) {
        String outputDir = args.length > 0 ? args[0] : "output";

        System.out.println("F5/James JPEG Reference Image Generator");
        System.out.println("========================================");
        System.out.println("Output directory: " + outputDir);
        System.out.println();

        // Create output directory
        File outDir = new File(outputDir);
        if (!outDir.exists()) {
            outDir.mkdirs();
        }

        // Create subdirectories for each subsampling mode
        // Note: The standalone encoder only supports 4:2:0 mode
        // For full compatibility testing, we generate 4:2:0 reference images
        String subsamplingDir = new File(outputDir, "4_2_0").getPath();
        new File(subsamplingDir).mkdirs();

        ReferenceGenerator generator = new ReferenceGenerator();
        int totalGenerated = 0;
        int totalErrors = 0;

        // Generate all combinations
        for (int[] dims : DIMENSIONS) {
            int width = dims[0];
            int height = dims[1];
            String dimStr = width + "x" + height;

            for (int patternType = 0; patternType < PATTERN_NAMES.length; patternType++) {
                String patternName = PATTERN_NAMES[patternType];

                for (int quality : QUALITY_LEVELS) {
                    try {
                        // Generate test image
                        BufferedImage img = generator.generatePattern(patternType, width, height);

                        // Encode to JPEG
                        ByteArrayOutputStream baos = new ByteArrayOutputStream();
                        JpegEncoder encoder = new JpegEncoder(img, quality, baos);
                        encoder.Compress();
                        byte[] jpegData = baos.toByteArray();

                        // Generate filename
                        String filename = String.format("%s_%s_q%02d_420.jpg",
                            patternName, dimStr, quality);
                        String filepath = new File(subsamplingDir, filename).getPath();

                        // Write file
                        try (FileOutputStream fos = new FileOutputStream(filepath)) {
                            fos.write(jpegData);
                        }

                        // Calculate SHA-256
                        String sha256 = calculateSHA256(jpegData);

                        System.out.printf("Generated: %s (%d bytes) SHA256: %s%n",
                            filename, jpegData.length, sha256.substring(0, 16) + "...");

                        totalGenerated++;
                    } catch (Exception e) {
                        System.err.printf("Error generating %s_%s_q%d: %s%n",
                            patternName, dimStr, quality, e.getMessage());
                        e.printStackTrace();
                        totalErrors++;
                    }
                }
            }
        }

        System.out.println();
        System.out.println("========================================");
        System.out.printf("Total generated: %d%n", totalGenerated);
        System.out.printf("Total errors: %d%n", totalErrors);

        // Generate manifest
        generateManifest(outputDir, subsamplingDir);
    }

    /**
     * Generate a test pattern image.
     */
    public BufferedImage generatePattern(int patternType, int width, int height) {
        BufferedImage img = new BufferedImage(width, height, BufferedImage.TYPE_INT_RGB);

        switch (patternType) {
            case PATTERN_SOLID:
                generateSolidPattern(img, width, height);
                break;
            case PATTERN_HORIZONTAL_GRADIENT:
                generateHorizontalGradient(img, width, height);
                break;
            case PATTERN_VERTICAL_GRADIENT:
                generateVerticalGradient(img, width, height);
                break;
            case PATTERN_DIAGONAL_GRADIENT:
                generateDiagonalGradient(img, width, height);
                break;
            case PATTERN_CHECKERBOARD:
                generateCheckerboard(img, width, height);
                break;
            case PATTERN_QUADRANT:
                generateQuadrant(img, width, height);
                break;
            default:
                throw new IllegalArgumentException("Unknown pattern type: " + patternType);
        }

        return img;
    }

    /**
     * Solid color pattern - medium gray (128, 128, 128).
     * Tests DC coefficient encoding with uniform blocks.
     */
    private void generateSolidPattern(BufferedImage img, int width, int height) {
        int rgb = new Color(128, 128, 128).getRGB();
        for (int y = 0; y < height; y++) {
            for (int x = 0; x < width; x++) {
                img.setRGB(x, y, rgb);
            }
        }
    }

    /**
     * Horizontal gradient - varies R from 0 to 255 across width.
     * Tests low-frequency horizontal content.
     */
    private void generateHorizontalGradient(BufferedImage img, int width, int height) {
        for (int y = 0; y < height; y++) {
            for (int x = 0; x < width; x++) {
                int r = (x * 255) / (width > 1 ? width - 1 : 1);
                int g = 128;
                int b = 128;
                img.setRGB(x, y, new Color(r, g, b).getRGB());
            }
        }
    }

    /**
     * Vertical gradient - varies G from 0 to 255 across height.
     * Tests low-frequency vertical content.
     */
    private void generateVerticalGradient(BufferedImage img, int width, int height) {
        for (int y = 0; y < height; y++) {
            int g = (y * 255) / (height > 1 ? height - 1 : 1);
            for (int x = 0; x < width; x++) {
                int r = 128;
                int b = 128;
                img.setRGB(x, y, new Color(r, g, b).getRGB());
            }
        }
    }

    /**
     * Diagonal gradient - varies B from 0 to 255 diagonally.
     * Tests diagonal frequency content.
     */
    private void generateDiagonalGradient(BufferedImage img, int width, int height) {
        int maxDist = width + height - 2;
        if (maxDist == 0) maxDist = 1;

        for (int y = 0; y < height; y++) {
            for (int x = 0; x < width; x++) {
                int dist = x + y;
                int b = (dist * 255) / maxDist;
                int r = 128;
                int g = 128;
                img.setRGB(x, y, new Color(r, g, b).getRGB());
            }
        }
    }

    /**
     * High-frequency checkerboard pattern - 8x8 blocks alternating black/white.
     * Tests high-frequency DCT coefficients.
     */
    private void generateCheckerboard(BufferedImage img, int width, int height) {
        int blockSize = 8;
        int white = new Color(255, 255, 255).getRGB();
        int black = new Color(0, 0, 0).getRGB();

        for (int y = 0; y < height; y++) {
            for (int x = 0; x < width; x++) {
                int blockX = x / blockSize;
                int blockY = y / blockSize;
                boolean isWhite = ((blockX + blockY) % 2) == 0;
                img.setRGB(x, y, isWhite ? white : black);
            }
        }
    }

    /**
     * Quadrant pattern - four different patterns in each quadrant.
     * Tests mixed content: gradient, checkerboard, solid, and stripes.
     */
    private void generateQuadrant(BufferedImage img, int width, int height) {
        int midX = width / 2;
        int midY = height / 2;

        for (int y = 0; y < height; y++) {
            for (int x = 0; x < width; x++) {
                int r, g, b;

                boolean left = x < midX;
                boolean top = y < midY;

                if (left && top) {
                    // Top-left: smooth gradient
                    r = (x * 255) / (midX > 0 ? midX : 1);
                    g = (y * 255) / (midY > 0 ? midY : 1);
                    b = 128;
                } else if (!left && top) {
                    // Top-right: high-frequency checkerboard (2x2 pixels)
                    boolean isWhite = ((x + y) % 2) == 0;
                    if (isWhite) {
                        r = g = b = 255;
                    } else {
                        r = g = b = 0;
                    }
                } else if (left && !top) {
                    // Bottom-left: vertical stripes
                    int stripeWidth = 8;
                    boolean isLight = ((x / stripeWidth) % 2) == 0;
                    if (isLight) {
                        r = g = b = 200;
                    } else {
                        r = g = b = 55;
                    }
                } else {
                    // Bottom-right: noise-like pattern (deterministic)
                    // Using a simple hash function for reproducibility
                    int noise = ((x * 7) + (y * 13) + (x * y)) % 256;
                    int base = ((x - midX) + (y - midY)) % 256;
                    r = (noise + base) / 2;
                    g = (256 - noise + base) / 2;
                    b = (noise + 256 - base) / 2;
                    // Clamp values
                    r = Math.max(0, Math.min(255, r));
                    g = Math.max(0, Math.min(255, g));
                    b = Math.max(0, Math.min(255, b));
                }

                img.setRGB(x, y, new Color(r, g, b).getRGB());
            }
        }
    }

    /**
     * Calculate SHA-256 hash of data.
     */
    private static String calculateSHA256(byte[] data) {
        try {
            MessageDigest digest = MessageDigest.getInstance("SHA-256");
            byte[] hash = digest.digest(data);
            StringBuilder sb = new StringBuilder();
            for (byte b : hash) {
                sb.append(String.format("%02x", b));
            }
            return sb.toString();
        } catch (NoSuchAlgorithmException e) {
            return "ERROR";
        }
    }

    /**
     * Generate manifest.sha256 file with checksums of all generated images.
     */
    private static void generateManifest(String outputDir, String subsamplingDir) {
        try {
            StringBuilder manifest = new StringBuilder();
            manifest.append("# SHA-256 manifest for Java reference images\n");
            manifest.append("# Generated by ReferenceGenerator.java\n\n");

            File dir = new File(subsamplingDir);
            File[] files = dir.listFiles((d, name) -> name.endsWith(".jpg"));

            if (files != null) {
                java.util.Arrays.sort(files);
                for (File file : files) {
                    byte[] data = readFile(file);
                    String sha256 = calculateSHA256(data);
                    manifest.append(String.format("%s  4_2_0/%s%n", sha256, file.getName()));
                }
            }

            // Write manifest
            String manifestPath = new File(outputDir, "manifest.sha256").getPath();
            try (FileWriter fw = new FileWriter(manifestPath)) {
                fw.write(manifest.toString());
            }

            System.out.println("\nGenerated manifest: " + manifestPath);
        } catch (IOException e) {
            System.err.println("Error generating manifest: " + e.getMessage());
        }
    }

    /**
     * Read a file into a byte array.
     */
    private static byte[] readFile(File file) throws IOException {
        try (FileInputStream fis = new FileInputStream(file)) {
            byte[] data = new byte[(int) file.length()];
            fis.read(data);
            return data;
        }
    }

    /**
     * Generate a single pattern at specific dimensions and quality.
     * Useful for debugging or single-image generation.
     */
    public static void generateSingle(String patternName, int width, int height,
                                       int quality, String outputPath) throws IOException {
        int patternType = -1;
        for (int i = 0; i < PATTERN_NAMES.length; i++) {
            if (PATTERN_NAMES[i].equals(patternName)) {
                patternType = i;
                break;
            }
        }

        if (patternType < 0) {
            throw new IllegalArgumentException("Unknown pattern: " + patternName);
        }

        ReferenceGenerator gen = new ReferenceGenerator();
        BufferedImage img = gen.generatePattern(patternType, width, height);

        ByteArrayOutputStream baos = new ByteArrayOutputStream();
        JpegEncoder encoder = new JpegEncoder(img, quality, baos);
        encoder.Compress();

        try (FileOutputStream fos = new FileOutputStream(outputPath)) {
            fos.write(baos.toByteArray());
        }

        System.out.println("Generated: " + outputPath);
    }
}
