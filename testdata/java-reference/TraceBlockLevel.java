import java.awt.Color;
import java.awt.image.BufferedImage;
import java.io.*;

public class TraceBlockLevel {
    public static void main(String[] args) throws Exception {
        // Create solid gray 64x64 image
        BufferedImage image = new BufferedImage(64, 64, BufferedImage.TYPE_INT_RGB);
        Color gray = new Color(128, 128, 128);
        for (int y = 0; y < 64; y++) {
            for (int x = 0; x < 64; x++) {
                image.setRGB(x, y, gray.getRGB());
            }
        }
        
        JpegInfo JpegObj = new JpegInfo(image);
        DCT dct = new DCT(75);
        
        int imageHeight = JpegObj.imageHeight;
        int imageWidth = JpegObj.imageWidth;
        
        // Simulating exactly what WriteCompressedData does
        int MinBlockWidth = imageWidth % 8 != 0 ?
            (int) (Math.floor(imageWidth / 8.0) + 1) * 8 : imageWidth;
        int MinBlockHeight = imageHeight % 8 != 0 ?
            (int) (Math.floor(imageHeight / 8.0) + 1) * 8 : imageHeight;
            
        for (int comp = 0; comp < JpegObj.NumberOfComponents; comp++) {
            MinBlockWidth = Math.min(MinBlockWidth, JpegObj.BlockWidth[comp]);
            MinBlockHeight = Math.min(MinBlockHeight, JpegObj.BlockHeight[comp]);
        }
        
        System.out.println("MinBlockWidth: " + MinBlockWidth);
        System.out.println("MinBlockHeight: " + MinBlockHeight);
        System.out.println("VsampFactor[0]: " + JpegObj.VsampFactor[0]);
        System.out.println("HsampFactor[0]: " + JpegObj.HsampFactor[0]);
        
        // First iteration: r=0, c=0, comp=0, i=0, j=0
        int r = 0, c = 0;
        int comp = 0;
        int i = 0, j = 0;
        int xpos = c * 8;
        int ypos = r * 8;
        int xblockoffset = j * 8;
        int yblockoffset = i * 8;
        
        System.out.println("\nFirst Y block extraction:");
        System.out.println("xpos=" + xpos + " ypos=" + ypos);
        System.out.println("xblockoffset=" + xblockoffset + " yblockoffset=" + yblockoffset);
        
        float[][] dctArray1 = new float[8][8];
        for (int a = 0; a < 8; a++) {
            for (int b = 0; b < 8; b++) {
                int ia = ypos * JpegObj.VsampFactor[comp] + yblockoffset + a;
                int ib = xpos * JpegObj.HsampFactor[comp] + xblockoffset + b;
                if (imageHeight / 2 * JpegObj.VsampFactor[comp] <= ia) {
                    ia = imageHeight / 2 * JpegObj.VsampFactor[comp] - 1;
                }
                if (imageWidth / 2 * JpegObj.HsampFactor[comp] <= ib) {
                    ib = imageWidth / 2 * JpegObj.HsampFactor[comp] - 1;
                }
                dctArray1[a][b] = JpegObj.getY(ia, ib);
            }
        }
        
        System.out.println("\nBlock values passed to DCT (NO level shift in WriteCompressedData):");
        for (int a = 0; a < 8; a++) {
            for (int b = 0; b < 8; b++) {
                System.out.printf("%.1f ", dctArray1[a][b]);
            }
            System.out.println();
        }
        
        // Now call the DCT
        double[][] dctArray2 = dct.forwardDCT(dctArray1);
        
        System.out.println("\nAfter DCT.forwardDCT():");
        for (int a = 0; a < 8; a++) {
            for (int b = 0; b < 8; b++) {
                System.out.printf("%.4f ", dctArray2[a][b]);
            }
            System.out.println();
        }
        
        // Quantize
        int[] dctArray3 = dct.quantizeBlock(dctArray2, JpegObj.QtableNumber[comp]);
        
        System.out.println("\nAfter quantization:");
        for (int idx = 0; idx < 64; idx++) {
            System.out.printf("%d ", dctArray3[idx]);
            if ((idx + 1) % 8 == 0) System.out.println();
        }
        
        System.out.println("\nFirst DC coefficient: " + dctArray3[0]);
    }
}
