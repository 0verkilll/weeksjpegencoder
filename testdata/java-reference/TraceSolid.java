import java.awt.Color;
import java.awt.image.BufferedImage;
import java.io.*;

public class TraceSolid {
    public static void main(String[] args) throws Exception {
        // Create solid gray 64x64 image
        BufferedImage image = new BufferedImage(64, 64, BufferedImage.TYPE_INT_RGB);
        Color gray = new Color(128, 128, 128);
        for (int y = 0; y < 64; y++) {
            for (int x = 0; x < 64; x++) {
                image.setRGB(x, y, gray.getRGB());
            }
        }
        
        // Create JpegInfo to see the YCbCr values
        JpegInfo info = new JpegInfo(image);
        
        System.out.println("Image dimensions: " + info.imageWidth + "x" + info.imageHeight);
        
        // Print first few Y values
        System.out.println("\nFirst Y values (using getY):");
        for (int y = 0; y < 8; y++) {
            for (int x = 0; x < 8; x++) {
                System.out.printf("%.1f ", info.getY(y, x));
            }
            System.out.println();
        }
        
        // Print first few Cb values
        System.out.println("\nFirst Cb values (using getCb):");
        for (int y = 0; y < 8; y++) {
            for (int x = 0; x < 8; x++) {
                System.out.printf("%.1f ", info.getCb(y, x));
            }
            System.out.println();
        }
        
        // Print first few Cr values
        System.out.println("\nFirst Cr values (using getCr):");
        for (int y = 0; y < 8; y++) {
            for (int x = 0; x < 8; x++) {
                System.out.printf("%.1f ", info.getCr(y, x));
            }
            System.out.println();
        }
        
        // Now trace DCT of first Y block
        DCT dct = new DCT(75);
        
        // Extract first Y block
        float[][] inputArray = new float[8][8];
        for (int i = 0; i < 8; i++) {
            for (int j = 0; j < 8; j++) {
                inputArray[i][j] = info.getY(i, j);
            }
        }
        
        System.out.println("\nFirst Y block (before level shift):");
        for (int i = 0; i < 8; i++) {
            for (int j = 0; j < 8; j++) {
                System.out.printf("%.1f ", inputArray[i][j]);
            }
            System.out.println();
        }
        
        // Level shift
        for (int i = 0; i < 8; i++) {
            for (int j = 0; j < 8; j++) {
                inputArray[i][j] -= 128.0f;
            }
        }
        
        System.out.println("\nFirst Y block (after level shift):");
        for (int i = 0; i < 8; i++) {
            for (int j = 0; j < 8; j++) {
                System.out.printf("%.1f ", inputArray[i][j]);
            }
            System.out.println();
        }
        
        // Do DCT
        double[][] dctResult = dct.forwardDCT(inputArray);
        
        System.out.println("\nAfter DCT:");
        for (int i = 0; i < 8; i++) {
            for (int j = 0; j < 8; j++) {
                System.out.printf("%.4f ", dctResult[i][j]);
            }
            System.out.println();
        }
        
        // Quantize
        int[] quantized = dct.quantizeBlock(dctResult, 0);  // 0 = luminance table
        
        System.out.println("\nQuantized coefficients (row-major):");
        for (int i = 0; i < 64; i++) {
            System.out.printf("%d ", quantized[i]);
            if ((i + 1) % 8 == 0) System.out.println();
        }
        
        // Check if all zeros
        boolean allZero = true;
        for (int i = 0; i < 64; i++) {
            if (quantized[i] != 0) {
                allZero = false;
                break;
            }
        }
        System.out.println("All zeros: " + allZero);
    }
}
