// Huffman - Standalone version based on James R. Weeks' implementation
// Copyright (C) 1998, James R. Weeks and BioElectroMech.
//
// Based on the Independent JPEG Group's work (Thomas G. Lane's Jpeg 6a library).

import java.io.*;
import java.util.*;

/**
 * Huffman - Performs Huffman encoding for JPEG compression.
 */
public class Huffman {
    private int bufferPutBits, bufferPutBuffer;
    private int imageHeight, imageWidth;

    // Huffman tables
    public ArrayList<int[]> bits = new ArrayList<int[]>();
    public ArrayList<int[]> val = new ArrayList<int[]>();

    // Huffman encoder tables (code and size)
    private int[][] DC_matrix0;
    private int[][] DC_matrix1;
    private int[][] AC_matrix0;
    private int[][] AC_matrix1;

    // Standard DC luminance Huffman table bits (ITU-T T.81 Annex K.3.3.1)
    private static final int[] DC_LUMINANCE_BITS = {
        0, 0, 1, 5, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0
    };
    // Standard DC luminance Huffman table values
    private static final int[] DC_LUMINANCE_VALUES = {
        0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11
    };

    // Standard DC chrominance Huffman table bits
    private static final int[] DC_CHROMINANCE_BITS = {
        0, 0, 3, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0
    };
    // Standard DC chrominance Huffman table values
    private static final int[] DC_CHROMINANCE_VALUES = {
        0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11
    };

    // Standard AC luminance Huffman table bits (ITU-T T.81 Annex K.3.3.2)
    private static final int[] AC_LUMINANCE_BITS = {
        0, 0, 2, 1, 3, 3, 2, 4, 3, 5, 5, 4, 4, 0, 0, 1, 125
    };
    // Standard AC luminance Huffman table values
    private static final int[] AC_LUMINANCE_VALUES = {
        0x01, 0x02, 0x03, 0x00, 0x04, 0x11, 0x05, 0x12,
        0x21, 0x31, 0x41, 0x06, 0x13, 0x51, 0x61, 0x07,
        0x22, 0x71, 0x14, 0x32, 0x81, 0x91, 0xa1, 0x08,
        0x23, 0x42, 0xb1, 0xc1, 0x15, 0x52, 0xd1, 0xf0,
        0x24, 0x33, 0x62, 0x72, 0x82, 0x09, 0x0a, 0x16,
        0x17, 0x18, 0x19, 0x1a, 0x25, 0x26, 0x27, 0x28,
        0x29, 0x2a, 0x34, 0x35, 0x36, 0x37, 0x38, 0x39,
        0x3a, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48, 0x49,
        0x4a, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58, 0x59,
        0x5a, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x69,
        0x6a, 0x73, 0x74, 0x75, 0x76, 0x77, 0x78, 0x79,
        0x7a, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88, 0x89,
        0x8a, 0x92, 0x93, 0x94, 0x95, 0x96, 0x97, 0x98,
        0x99, 0x9a, 0xa2, 0xa3, 0xa4, 0xa5, 0xa6, 0xa7,
        0xa8, 0xa9, 0xaa, 0xb2, 0xb3, 0xb4, 0xb5, 0xb6,
        0xb7, 0xb8, 0xb9, 0xba, 0xc2, 0xc3, 0xc4, 0xc5,
        0xc6, 0xc7, 0xc8, 0xc9, 0xca, 0xd2, 0xd3, 0xd4,
        0xd5, 0xd6, 0xd7, 0xd8, 0xd9, 0xda, 0xe1, 0xe2,
        0xe3, 0xe4, 0xe5, 0xe6, 0xe7, 0xe8, 0xe9, 0xea,
        0xf1, 0xf2, 0xf3, 0xf4, 0xf5, 0xf6, 0xf7, 0xf8,
        0xf9, 0xfa
    };

    // Standard AC chrominance Huffman table bits
    private static final int[] AC_CHROMINANCE_BITS = {
        0, 0, 2, 1, 2, 4, 4, 3, 4, 7, 5, 4, 4, 0, 1, 2, 119
    };
    // Standard AC chrominance Huffman table values
    private static final int[] AC_CHROMINANCE_VALUES = {
        0x00, 0x01, 0x02, 0x03, 0x11, 0x04, 0x05, 0x21,
        0x31, 0x06, 0x12, 0x41, 0x51, 0x07, 0x61, 0x71,
        0x13, 0x22, 0x32, 0x81, 0x08, 0x14, 0x42, 0x91,
        0xa1, 0xb1, 0xc1, 0x09, 0x23, 0x33, 0x52, 0xf0,
        0x15, 0x62, 0x72, 0xd1, 0x0a, 0x16, 0x24, 0x34,
        0xe1, 0x25, 0xf1, 0x17, 0x18, 0x19, 0x1a, 0x26,
        0x27, 0x28, 0x29, 0x2a, 0x35, 0x36, 0x37, 0x38,
        0x39, 0x3a, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48,
        0x49, 0x4a, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58,
        0x59, 0x5a, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68,
        0x69, 0x6a, 0x73, 0x74, 0x75, 0x76, 0x77, 0x78,
        0x79, 0x7a, 0x82, 0x83, 0x84, 0x85, 0x86, 0x87,
        0x88, 0x89, 0x8a, 0x92, 0x93, 0x94, 0x95, 0x96,
        0x97, 0x98, 0x99, 0x9a, 0xa2, 0xa3, 0xa4, 0xa5,
        0xa6, 0xa7, 0xa8, 0xa9, 0xaa, 0xb2, 0xb3, 0xb4,
        0xb5, 0xb6, 0xb7, 0xb8, 0xb9, 0xba, 0xc2, 0xc3,
        0xc4, 0xc5, 0xc6, 0xc7, 0xc8, 0xc9, 0xca, 0xd2,
        0xd3, 0xd4, 0xd5, 0xd6, 0xd7, 0xd8, 0xd9, 0xda,
        0xe2, 0xe3, 0xe4, 0xe5, 0xe6, 0xe7, 0xe8, 0xe9,
        0xea, 0xf2, 0xf3, 0xf4, 0xf5, 0xf6, 0xf7, 0xf8,
        0xf9, 0xfa
    };

    public Huffman(int width, int height) {
        imageWidth = width;
        imageHeight = height;
        bufferPutBits = 0;
        bufferPutBuffer = 0;

        // Initialize bits and val vectors
        bits.add(DC_LUMINANCE_BITS);
        bits.add(AC_LUMINANCE_BITS);
        bits.add(DC_CHROMINANCE_BITS);
        bits.add(AC_CHROMINANCE_BITS);

        val.add(DC_LUMINANCE_VALUES);
        val.add(AC_LUMINANCE_VALUES);
        val.add(DC_CHROMINANCE_VALUES);
        val.add(AC_CHROMINANCE_VALUES);

        initHuf();
    }

    private void initHuf() {
        DC_matrix0 = makeEncoderTable(DC_LUMINANCE_BITS, DC_LUMINANCE_VALUES);
        DC_matrix1 = makeEncoderTable(DC_CHROMINANCE_BITS, DC_CHROMINANCE_VALUES);
        AC_matrix0 = makeEncoderTable(AC_LUMINANCE_BITS, AC_LUMINANCE_VALUES);
        AC_matrix1 = makeEncoderTable(AC_CHROMINANCE_BITS, AC_CHROMINANCE_VALUES);
    }

    private int[][] makeEncoderTable(int[] nrcodes, int[] values) {
        int[][] table = new int[2][256];

        int code = 0;
        int k = 0;
        for (int i = 1; i <= 16; i++) {
            for (int j = 0; j < nrcodes[i]; j++) {
                table[0][values[k]] = code;
                table[1][values[k]] = i;
                code++;
                k++;
            }
            code <<= 1;
        }

        return table;
    }

    private void bufferIt(BufferedOutputStream out, int code, int size) throws IOException {
        int putBuffer = code;
        int putBits = bufferPutBits;

        putBuffer &= (1 << size) - 1;
        putBits += size;
        putBuffer <<= 24 - putBits;
        putBuffer |= bufferPutBuffer;

        while (putBits >= 8) {
            int c = ((putBuffer >> 16) & 0xFF);
            out.write(c);
            if (c == 0xFF) {
                out.write(0);  // Byte stuffing
            }
            putBuffer <<= 8;
            putBits -= 8;
        }

        bufferPutBuffer = putBuffer;
        bufferPutBits = putBits;
    }

    public void flushBuffer(BufferedOutputStream out) throws IOException {
        int putBuffer = bufferPutBuffer;
        int putBits = bufferPutBits;

        while (putBits >= 8) {
            int c = ((putBuffer >> 16) & 0xFF);
            out.write(c);
            if (c == 0xFF) {
                out.write(0);
            }
            putBuffer <<= 8;
            putBits -= 8;
        }

        if (putBits > 0) {
            int c = ((putBuffer >> 16) & 0xFF);
            out.write(c);
        }
    }

    public void HuffmanBlockEncoder(BufferedOutputStream out, int[] dctArray,
                                    int lastDC, int dcTable, int acTable) throws IOException {
        int[][] DCcode, ACcode;

        if (dcTable == 0) {
            DCcode = DC_matrix0;
        } else {
            DCcode = DC_matrix1;
        }

        if (acTable == 0) {
            ACcode = AC_matrix0;
        } else {
            ACcode = AC_matrix1;
        }

        // Encode DC coefficient
        int temp = dctArray[0] - lastDC;
        int temp2 = temp;
        if (temp < 0) {
            temp = -temp;
            temp2--;
        }

        int nbits = 0;
        while (temp != 0) {
            nbits++;
            temp >>= 1;
        }

        // Output Huffman code for DC category
        bufferIt(out, DCcode[0][nbits], DCcode[1][nbits]);

        // Output DC value
        if (nbits != 0) {
            bufferIt(out, temp2 & ((1 << nbits) - 1), nbits);
        }

        // Encode AC coefficients
        int r = 0;
        for (int k = 1; k < 64; k++) {
            temp = dctArray[JpegEncoder.jpegNaturalOrder[k]];

            if (temp == 0) {
                r++;
            } else {
                while (r > 15) {
                    bufferIt(out, ACcode[0][0xF0], ACcode[1][0xF0]);
                    r -= 16;
                }

                temp2 = temp;
                if (temp < 0) {
                    temp = -temp;
                    temp2--;
                }

                nbits = 0;
                while (temp != 0) {
                    nbits++;
                    temp >>= 1;
                }

                int i = (r << 4) + nbits;
                bufferIt(out, ACcode[0][i], ACcode[1][i]);
                bufferIt(out, temp2 & ((1 << nbits) - 1), nbits);

                r = 0;
            }
        }

        // EOB
        if (r > 0) {
            bufferIt(out, ACcode[0][0], ACcode[1][0]);
        }
    }
}
