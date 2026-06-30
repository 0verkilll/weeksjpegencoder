import java.awt.Color;
import java.awt.image.BufferedImage;
import java.io.*;

public class TraceQ100 {
    public static void main(String[] args) throws Exception {
        // Create horizontal gradient 8x8 image
        BufferedImage image = new BufferedImage(8, 8, BufferedImage.TYPE_INT_RGB);
        for (int y = 0; y < 8; y++) {
            for (int x = 0; x < 8; x++) {
                int r = (x * 255) / 7;  // 0 to 255
                image.setRGB(x, y, new Color(r, 128, 128).getRGB());
            }
        }
        
        JpegInfo JpegObj = new JpegInfo(image);
        DCT dct = new DCT(100);  // Q100
        
        float[][] dctArray1 = new float[8][8];
        // Extract Y component directly (simplified)
        for (int a = 0; a < 8; a++) {
            for (int b = 0; b < 8; b++) {
                dctArray1[a][b] = JpegObj.getY(a, b);
            }
        }
        
        System.out.println("Block values (Y component of gradient):");
        for (int a = 0; a < 8; a++) {
            for (int b = 0; b < 8; b++) {
                System.out.printf("%.1f ", dctArray1[a][b]);
            }
            System.out.println();
        }
        
        double[][] dctArray2 = dct.forwardDCT(dctArray1);
        
        System.out.println("\nAfter DCT.forwardDCT():");
        for (int a = 0; a < 8; a++) {
            for (int b = 0; b < 8; b++) {
                System.out.printf("%.6f ", dctArray2[a][b]);
            }
            System.out.println();
        }
        
        // Quantize
        int[] dctArray3 = dct.quantizeBlock(dctArray2, 0);  // Y uses table 0
        
        System.out.println("\nAfter quantization (Q100):");
        for (int idx = 0; idx < 64; idx++) {
            System.out.printf("%d ", dctArray3[idx]);
            if ((idx + 1) % 8 == 0) System.out.println();
        }
        
        // Print divisors for Q100
        System.out.println("\nDivisors[0] (first 8):");
        for (int i = 0; i < 8; i++) {
            System.out.printf("%.10f ", dct.divisors[0][i]);
        }
        System.out.println();
        
        // Print quantization table
        int[] qt = (int[]) dct.quantum[0];
        System.out.println("\nQuantization table [0] (first 8):");
        for (int i = 0; i < 8; i++) {
            System.out.printf("%d ", qt[i]);
        }
        System.out.println();
    }
}
