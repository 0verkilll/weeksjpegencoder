// JarDriver — encodes a raw RGB pixel buffer with f5.jar's james.JpegEncoder
// and writes the JPEG to disk. Used to produce byte-for-byte reference output
// straight from the original f5.jar (not the standalone Java port).
//
// Input format (read from inputPath):
//   4 bytes width  (big-endian int32)
//   4 bytes height (big-endian int32)
//   width*height * 4 bytes — each pixel is 0xAARRGGBB big-endian int32
//     (A is ignored; matches what MemoryImageSource expects)

import java.awt.Image;
import java.awt.Toolkit;
import java.awt.image.MemoryImageSource;
import java.io.DataInputStream;
import java.io.FileInputStream;
import java.io.FileOutputStream;

import james.JpegEncoder;

public class JarDriver {
    private static final String DEFAULT_COMMENT =
        "JPEG Encoder Copyright 1998, James R. Weeks and BioElectroMech.  ";

    public static void main(String[] args) throws Exception {
        if (args.length != 3) {
            System.err.println("Usage: JarDriver <input.raw> <output.jpg> <quality 1-100>");
            System.exit(2);
        }
        String inputPath = args[0];
        String outputPath = args[1];
        int quality = Integer.parseInt(args[2]);

        DataInputStream in = new DataInputStream(new FileInputStream(inputPath));
        int width = in.readInt();
        int height = in.readInt();
        int[] pixels = new int[width * height];
        for (int i = 0; i < pixels.length; i++) {
            pixels[i] = in.readInt();
        }
        in.close();

        MemoryImageSource source = new MemoryImageSource(width, height, pixels, 0, width);
        Image image = Toolkit.getDefaultToolkit().createImage(source);

        FileOutputStream out = new FileOutputStream(outputPath);
        JpegEncoder encoder = new JpegEncoder(image, quality, out, DEFAULT_COMMENT);
        encoder.Compress();
        out.close();
    }
}
