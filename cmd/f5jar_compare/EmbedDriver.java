// EmbedDriver — F5-embeds a message into a raw RGB pixel buffer using f5.jar's
// james.JpegEncoder.Compress(embeddedData, password) and writes the stego JPEG.
// This is the embed counterpart of JarDriver (which only encodes). Used to
// produce byte-for-byte reference STEGO output straight from the original f5.jar
// so the Go f5messageembed can be validated coefficient-for-coefficient.
//
// Input .raw format matches JarDriver:
//   4 bytes width (BE int32), 4 bytes height (BE int32),
//   width*height * 4 bytes — each pixel 0xAARRGGBB BE int32 (A ignored).
//
// Usage: EmbedDriver <input.raw> <output.jpg> <quality> <password> <message-file>

import java.awt.Image;
import java.awt.Toolkit;
import java.awt.image.MemoryImageSource;
import java.io.ByteArrayInputStream;
import java.io.DataInputStream;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.nio.file.Files;
import java.nio.file.Paths;

import james.JpegEncoder;

public class EmbedDriver {
    private static final String DEFAULT_COMMENT =
        "JPEG Encoder Copyright 1998, James R. Weeks and BioElectroMech.  ";

    // f5.jar's net.f5.crypt.F5Random seeds via `new SecureRandom(seed)`, whose
    // algorithm is the JVM default. On the JVMs F5 was written for, that default
    // was SHA1PRNG (deterministic from the seed). On Java 9+ the default became
    // NativePRNG, which IGNORES the seed and draws from OS entropy — making f5.jar
    // non-deterministic and impossible to byte-compare. We restore the original
    // contract by inserting a provider at position 1 that resolves the default
    // SecureRandom to the SHA1PRNG implementation. (Requires
    // --add-opens=java.base/sun.security.provider=ALL-UNNAMED.)
    private static void forceSha1PrngDefault() {
        java.security.Provider p =
            new java.security.Provider("Sha1First", "1.0", "force SHA1PRNG default") {};
        p.put("SecureRandom.SHA1PRNG", "sun.security.provider.SecureRandom");
        java.security.Security.insertProviderAt(p, 1);
    }

    public static void main(String[] args) throws Exception {
        forceSha1PrngDefault();
        if (args.length != 5) {
            System.err.println("Usage: EmbedDriver <input.raw> <output.jpg> <quality> <password> <message-file>");
            System.exit(2);
        }
        String inputPath = args[0];
        String outputPath = args[1];
        int quality = Integer.parseInt(args[2]);
        String password = args[3];
        byte[] msg = Files.readAllBytes(Paths.get(args[4]));

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
        encoder.Compress(new ByteArrayInputStream(msg), password);
        out.close();
    }
}
