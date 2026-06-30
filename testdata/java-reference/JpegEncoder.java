// JpegEncoder - Standalone version based on James R. Weeks' implementation
// Version 1.0a
// Copyright (C) 1998, James R. Weeks and BioElectroMech.
// Visit BioElectroMech at www.obrador.com. Email James@obrador.com.
//
// This standalone version removes Android dependencies for reference image generation.
// Based on the F5Android implementation at:
// https://github.com/guardianproject/F5Android/blob/1150c6418c14c36120613eb46ebb6075ca7302ac/src/main/java/info/guardianproject/f5android/plugins/f5/james/JpegEncoder.java

import java.awt.image.BufferedImage;
import java.io.*;

/**
 * JpegEncoder - The JPEG main program which performs a jpeg compression
 * of an image.
 */
public class JpegEncoder {
    private BufferedOutputStream outStream;
    private JpegInfo JpegObj;
    private Huffman Huf;
    private DCT dct;
    private int imageHeight, imageWidth;
    private int Quality;

    public static int[] jpegNaturalOrder = {
        0, 1, 8, 16, 9, 2, 3, 10, 17, 24, 32, 25, 18, 11, 4, 5, 12, 19, 26,
        33, 40, 48, 41, 34, 27, 20, 13, 6, 7, 14, 21, 28, 35, 42, 49, 56,
        57, 50, 43, 36, 29, 22, 15, 23, 30, 37, 44, 51, 58, 59, 52, 45, 38,
        31, 39, 46, 53, 60, 61, 54, 47, 55, 62, 63
    };

    public JpegEncoder(BufferedImage image, int quality, OutputStream out) {
        this.Quality = quality;
        this.JpegObj = new JpegInfo(image);
        this.imageHeight = this.JpegObj.imageHeight;
        this.imageWidth = this.JpegObj.imageWidth;
        this.outStream = new BufferedOutputStream(out);
        this.dct = new DCT(this.Quality);
        this.Huf = new Huffman(this.imageWidth, this.imageHeight);
    }

    public void Compress() throws IOException {
        WriteHeaders(this.outStream);
        WriteCompressedData(this.outStream);
        WriteEOI(this.outStream);
        this.outStream.flush();
    }

    public int getQuality() {
        return this.Quality;
    }

    public void setQuality(int quality) {
        this.dct = new DCT(quality);
    }

    void WriteArray(byte[] data, BufferedOutputStream out) throws IOException {
        int length = ((data[2] & 0xFF) << 8) + (data[3] & 0xFF) + 2;
        out.write(data, 0, length);
    }

    public void WriteCompressedData(BufferedOutputStream outStream) throws IOException {
        int i, j, r, c, a, b;
        int comp, xpos, ypos, xblockoffset, yblockoffset;
        float[][] dctArray1 = new float[8][8];
        double[][] dctArray2 = new double[8][8];
        int[] dctArray3 = new int[8 * 8];

        int[] lastDCvalue = new int[JpegObj.NumberOfComponents];
        int Width = 0, Height = 0;
        int MinBlockWidth, MinBlockHeight;

        MinBlockWidth = imageWidth % 8 != 0 ?
            (int) (Math.floor(imageWidth / 8.0) + 1) * 8 : imageWidth;
        MinBlockHeight = imageHeight % 8 != 0 ?
            (int) (Math.floor(imageHeight / 8.0) + 1) * 8 : imageHeight;

        for (comp = 0; comp < JpegObj.NumberOfComponents; comp++) {
            MinBlockWidth = Math.min(MinBlockWidth, JpegObj.BlockWidth[comp]);
            MinBlockHeight = Math.min(MinBlockHeight, JpegObj.BlockHeight[comp]);
        }

        xpos = 0;

        for (r = 0; r < MinBlockHeight; r++) {
            for (c = 0; c < MinBlockWidth; c++) {
                xpos = c * 8;
                ypos = r * 8;
                for (comp = 0; comp < JpegObj.NumberOfComponents; comp++) {
                    Width = JpegObj.BlockWidth[comp];
                    Height = JpegObj.BlockHeight[comp];

                    for (i = 0; i < JpegObj.VsampFactor[comp]; i++) {
                        for (j = 0; j < JpegObj.HsampFactor[comp]; j++) {
                            xblockoffset = j * 8;
                            yblockoffset = i * 8;
                            for (a = 0; a < 8; a++) {
                                for (b = 0; b < 8; b++) {
                                    int ia = ypos * JpegObj.VsampFactor[comp]
                                        + yblockoffset + a;
                                    int ib = xpos * JpegObj.HsampFactor[comp]
                                        + xblockoffset + b;
                                    if (imageHeight / 2 * JpegObj.VsampFactor[comp] <= ia) {
                                        ia = imageHeight / 2 * JpegObj.VsampFactor[comp] - 1;
                                    }
                                    if (imageWidth / 2 * JpegObj.HsampFactor[comp] <= ib) {
                                        ib = imageWidth / 2 * JpegObj.HsampFactor[comp] - 1;
                                    }

                                    switch(comp) {
                                    case 0:
                                        dctArray1[a][b] = JpegObj.getY(ia, ib);
                                        break;
                                    case 1:
                                        dctArray1[a][b] = JpegObj.getCb(ia, ib);
                                        break;
                                    case 2:
                                        dctArray1[a][b] = JpegObj.getCr(ia, ib);
                                        break;
                                    }
                                }
                            }

                            dctArray2 = dct.forwardDCT(dctArray1);
                            dctArray3 = dct.quantizeBlock(dctArray2,
                                JpegObj.QtableNumber[comp]);

                            Huf.HuffmanBlockEncoder(outStream, dctArray3,
                                lastDCvalue[comp],
                                JpegObj.DCtableNumber[comp],
                                JpegObj.ACtableNumber[comp]);
                            lastDCvalue[comp] = dctArray3[0];
                        }
                    }
                }
            }
        }

        Huf.flushBuffer(outStream);
    }

    public void WriteEOI(BufferedOutputStream out) throws IOException {
        byte[] EOI = { (byte) 0xFF, (byte) 0xD9 };
        WriteMarker(EOI, out);
    }

    public void WriteHeaders(BufferedOutputStream out) throws IOException {
        int i, j, index, offset;
        int[] tempArray;

        // SOI
        byte[] SOI = { (byte) 0xFF, (byte) 0xD8 };
        WriteMarker(SOI, out);

        // APP0/JFIF
        byte[] JFIF = new byte[18];
        JFIF[0] = (byte) 0xff;
        JFIF[1] = (byte) 0xe0;
        JFIF[2] = (byte) 0x00;
        JFIF[3] = (byte) 0x10;
        JFIF[4] = (byte) 0x4a;  // J
        JFIF[5] = (byte) 0x46;  // F
        JFIF[6] = (byte) 0x49;  // I
        JFIF[7] = (byte) 0x46;  // F
        JFIF[8] = (byte) 0x00;
        JFIF[9] = (byte) 0x01;  // Version 1.01
        JFIF[10] = (byte) 0x01;
        JFIF[11] = (byte) 0x00; // Density units: 0 = no units
        JFIF[12] = (byte) 0x00; // X density high byte
        JFIF[13] = (byte) 0x01; // X density low byte
        JFIF[14] = (byte) 0x00; // Y density high byte
        JFIF[15] = (byte) 0x01; // Y density low byte
        JFIF[16] = (byte) 0x00; // No thumbnail
        JFIF[17] = (byte) 0x00;
        WriteArray(JFIF, out);

        // COM - Comment marker
        String comment = "JPEG Encoder Copyright 1998, James R. Weeks and BioElectroMech.";
        byte[] commentBytes = comment.getBytes("ISO-8859-1");
        byte[] COM = new byte[4 + commentBytes.length];
        COM[0] = (byte) 0xFF;
        COM[1] = (byte) 0xFE;
        int comLength = commentBytes.length + 2;
        COM[2] = (byte) ((comLength >> 8) & 0xFF);
        COM[3] = (byte) (comLength & 0xFF);
        System.arraycopy(commentBytes, 0, COM, 4, commentBytes.length);
        out.write(COM);

        // DQT - Quantization tables
        byte[] DQT = new byte[134];
        DQT[0] = (byte) 0xFF;
        DQT[1] = (byte) 0xDB;
        DQT[2] = (byte) 0x00;
        DQT[3] = (byte) 0x84;
        offset = 4;
        for (i = 0; i < 2; i++) {
            DQT[offset++] = (byte) ((0 << 4) + i);
            tempArray = (int[]) dct.quantum[i];
            for (j = 0; j < 64; j++) {
                DQT[offset++] = (byte) tempArray[jpegNaturalOrder[j]];
            }
        }
        WriteArray(DQT, out);

        // SOF0 - Frame header
        byte[] SOF = new byte[19];
        SOF[0] = (byte) 0xFF;
        SOF[1] = (byte) 0xC0;
        SOF[2] = (byte) 0x00;
        SOF[3] = (byte) 17;
        SOF[4] = (byte) JpegObj.Precision;
        SOF[5] = (byte) ((JpegObj.imageHeight >> 8) & 0xFF);
        SOF[6] = (byte) (JpegObj.imageHeight & 0xFF);
        SOF[7] = (byte) ((JpegObj.imageWidth >> 8) & 0xFF);
        SOF[8] = (byte) (JpegObj.imageWidth & 0xFF);
        SOF[9] = (byte) JpegObj.NumberOfComponents;
        index = 10;
        for (i = 0; i < SOF[9]; i++) {
            SOF[index++] = (byte) JpegObj.CompID[i];
            SOF[index++] = (byte) ((JpegObj.HsampFactor[i] << 4)
                + JpegObj.VsampFactor[i]);
            SOF[index++] = (byte) JpegObj.QtableNumber[i];
        }
        WriteArray(SOF, out);

        // DHT - Huffman tables
        byte[] DHT1, DHT2, DHT3, DHT4;
        int bytes, temp, oldindex, intermediateindex;
        int length = 2;
        index = 4;
        oldindex = 4;
        DHT1 = new byte[17];
        DHT4 = new byte[4];
        DHT4[0] = (byte) 0xFF;
        DHT4[1] = (byte) 0xC4;
        for (i = 0; i < 4; i++) {
            bytes = 0;
            DHT1[index++ - oldindex] = (byte) Huf.bits.get(i)[0];
            for (j = 1; j < 17; j++) {
                temp = Huf.bits.get(i)[j];
                DHT1[index++ - oldindex] = (byte) temp;
                bytes += temp;
            }
            intermediateindex = index;
            DHT2 = new byte[bytes];
            for (j = 0; j < bytes; j++) {
                DHT2[index++ - intermediateindex] = (byte) Huf.val.get(i)[j];
            }
            DHT3 = new byte[index];
            System.arraycopy(DHT4, 0, DHT3, 0, oldindex);
            System.arraycopy(DHT1, 0, DHT3, oldindex, 17);
            System.arraycopy(DHT2, 0, DHT3, oldindex + 17, bytes);
            DHT4 = DHT3;
            oldindex = index;
        }
        DHT4[2] = (byte) ((index - 2 >> 8) & 0xFF);
        DHT4[3] = (byte) ((index - 2) & 0xFF);
        WriteArray(DHT4, out);

        // SOS - Scan header
        byte[] SOS = new byte[14];
        SOS[0] = (byte) 0xFF;
        SOS[1] = (byte) 0xDA;
        SOS[2] = (byte) 0x00;
        SOS[3] = (byte) 12;
        SOS[4] = (byte) JpegObj.NumberOfComponents;
        index = 5;
        for (i = 0; i < SOS[4]; i++) {
            SOS[index++] = (byte) JpegObj.CompID[i];
            SOS[index++] = (byte) ((JpegObj.DCtableNumber[i] << 4)
                + JpegObj.ACtableNumber[i]);
        }
        SOS[index++] = (byte) JpegObj.Ss;
        SOS[index++] = (byte) JpegObj.Se;
        SOS[index++] = (byte) ((JpegObj.Ah << 4) + JpegObj.Al);
        WriteArray(SOS, out);
    }

    void WriteMarker(byte[] data, BufferedOutputStream out) throws IOException {
        out.write(data, 0, 2);
    }
}
