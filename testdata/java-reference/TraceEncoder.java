import java.awt.Color;
import java.awt.image.BufferedImage;
import java.io.*;

public class TraceEncoder {
    public static void main(String[] args) throws Exception {
        // Create solid gray 64x64 image
        BufferedImage image = new BufferedImage(64, 64, BufferedImage.TYPE_INT_RGB);
        Color gray = new Color(128, 128, 128);
        for (int y = 0; y < 64; y++) {
            for (int x = 0; x < 64; x++) {
                image.setRGB(x, y, gray.getRGB());
            }
        }
        
        // Encode to JPEG and capture output
        ByteArrayOutputStream baos = new ByteArrayOutputStream();
        JpegEncoder encoder = new JpegEncoder(image, 75, baos);
        encoder.Compress();
        byte[] jpegData = baos.toByteArray();
        
        System.out.println("Encoded size: " + jpegData.length + " bytes");
        
        // Find entropy data start (after SOS marker)
        int entropyStart = -1;
        for (int i = 0; i < jpegData.length - 2; i++) {
            if ((jpegData[i] & 0xFF) == 0xFF && (jpegData[i+1] & 0xFF) == 0xDA) {
                // SOS marker found
                int sosLen = ((jpegData[i+2] & 0xFF) << 8) | (jpegData[i+3] & 0xFF);
                entropyStart = i + 2 + sosLen;
                break;
            }
        }
        
        System.out.println("Entropy data starts at offset: " + entropyStart);
        
        // Print first 64 bytes of entropy data
        System.out.println("\nFirst 64 bytes of entropy data:");
        for (int i = 0; i < 64 && entropyStart + i < jpegData.length; i++) {
            System.out.printf("%02x ", jpegData[entropyStart + i] & 0xFF);
            if ((i + 1) % 16 == 0) System.out.println();
        }
        System.out.println();
        
        // Write to file
        FileOutputStream fos = new FileOutputStream("/tmp/java_solid_test.jpg");
        fos.write(jpegData);
        fos.close();
        System.out.println("Written to /tmp/java_solid_test.jpg");
    }
}
