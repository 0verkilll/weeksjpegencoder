// JpegInfo - Standalone version based on James R. Weeks' implementation
// Copyright (C) 1998, James R. Weeks and BioElectroMech.
//
// This standalone version removes Android dependencies for reference image generation.

import java.awt.image.BufferedImage;

/**
 * JpegInfo - Given an image, generates all the information necessary to
 * produce a JPEG file.
 */
public class JpegInfo {
    public int imageHeight;
    public int imageWidth;
    public int Precision = 8;
    public int NumberOfComponents = 3;

    // Component specs
    public int[] CompID = { 1, 2, 3 };
    public int[] HsampFactor = { 2, 1, 1 };  // 4:2:0 default
    public int[] VsampFactor = { 2, 1, 1 };
    public int[] QtableNumber = { 0, 1, 1 };
    public int[] DCtableNumber = { 0, 1, 1 };
    public int[] ACtableNumber = { 0, 1, 1 };

    // Block dimensions
    public int[] BlockWidth;
    public int[] BlockHeight;

    // Spectral selection
    public int Ss = 0;
    public int Se = 63;
    public int Ah = 0;
    public int Al = 0;

    // YCbCr color components
    private float[][] Y;
    private float[][] Cb;
    private float[][] Cr;

    public JpegInfo(BufferedImage image) {
        this.imageWidth = image.getWidth();
        this.imageHeight = image.getHeight();

        // Calculate block dimensions
        BlockWidth = new int[NumberOfComponents];
        BlockHeight = new int[NumberOfComponents];

        for (int i = 0; i < NumberOfComponents; i++) {
            BlockWidth[i] = (int) Math.ceil((double) imageWidth / 8.0 / HsampFactor[i])
                * HsampFactor[i];
            BlockHeight[i] = (int) Math.ceil((double) imageHeight / 8.0 / VsampFactor[i])
                * VsampFactor[i];
        }

        // Convert RGB to YCbCr
        convertToYCbCr(image);
    }

    /**
     * Set subsampling mode.
     * 0 = 4:2:0 (default)
     * 1 = 4:2:2
     * 2 = 4:4:4
     */
    public void setSubsampling(int mode) {
        switch (mode) {
            case 0: // 4:2:0
                HsampFactor = new int[] { 2, 1, 1 };
                VsampFactor = new int[] { 2, 1, 1 };
                break;
            case 1: // 4:2:2
                HsampFactor = new int[] { 2, 1, 1 };
                VsampFactor = new int[] { 1, 1, 1 };
                break;
            case 2: // 4:4:4
                HsampFactor = new int[] { 1, 1, 1 };
                VsampFactor = new int[] { 1, 1, 1 };
                break;
        }
        // Recalculate block dimensions
        for (int i = 0; i < NumberOfComponents; i++) {
            BlockWidth[i] = (int) Math.ceil((double) imageWidth / 8.0 / HsampFactor[i])
                * HsampFactor[i];
            BlockHeight[i] = (int) Math.ceil((double) imageHeight / 8.0 / VsampFactor[i])
                * VsampFactor[i];
        }
    }

    private void convertToYCbCr(BufferedImage image) {
        int width = imageWidth;
        int height = imageHeight;

        // Allocate arrays for full resolution
        int maxH = 2;  // Max horizontal factor for 4:2:0
        int maxV = 2;  // Max vertical factor for 4:2:0

        int yWidth = (int) Math.ceil((double) width / 8.0 / maxH) * maxH * 8;
        int yHeight = (int) Math.ceil((double) height / 8.0 / maxV) * maxV * 8;

        Y = new float[yHeight][yWidth];
        Cb = new float[yHeight / 2][yWidth / 2];
        Cr = new float[yHeight / 2][yWidth / 2];

        // Convert each pixel
        for (int y = 0; y < height; y++) {
            for (int x = 0; x < width; x++) {
                int rgb = image.getRGB(x, y);
                int r = (rgb >> 16) & 0xFF;
                int g = (rgb >> 8) & 0xFF;
                int b = rgb & 0xFF;

                // BT.601 RGB to YCbCr conversion
                // Y  =  0.299R + 0.587G + 0.114B
                // Cb = -0.168736R - 0.331264G + 0.5B + 128
                // Cr =  0.5R - 0.418688G - 0.081312B + 128
                float yVal = (float) (0.299 * r + 0.587 * g + 0.114 * b);
                float cbVal = (float) (-0.168736 * r - 0.331264 * g + 0.5 * b + 128);
                float crVal = (float) (0.5 * r - 0.418688 * g - 0.081312 * b + 128);

                Y[y][x] = yVal;

                // Subsample Cb and Cr (2x2 averaging for 4:2:0)
                int cx = x / 2;
                int cy = y / 2;
                if (cx < Cb[0].length && cy < Cb.length) {
                    if (x % 2 == 0 && y % 2 == 0) {
                        Cb[cy][cx] = cbVal;
                        Cr[cy][cx] = crVal;
                    }
                }
            }
        }

        // Pad Y to block boundaries
        for (int y = height; y < yHeight; y++) {
            for (int x = 0; x < yWidth; x++) {
                int srcX = Math.min(x, width - 1);
                int srcY = height - 1;
                Y[y][x] = Y[srcY][srcX];
            }
        }
        for (int y = 0; y < height; y++) {
            for (int x = width; x < yWidth; x++) {
                Y[y][x] = Y[y][width - 1];
            }
        }

        // Pad Cb and Cr
        int cbWidth = Cb[0].length;
        int cbHeight = Cb.length;
        for (int y = (height + 1) / 2; y < cbHeight; y++) {
            for (int x = 0; x < cbWidth; x++) {
                int srcY = cbHeight - 1;
                if (srcY >= (height + 1) / 2) srcY = (height + 1) / 2 - 1;
                Cb[y][x] = Cb[srcY][Math.min(x, cbWidth - 1)];
                Cr[y][x] = Cr[srcY][Math.min(x, cbWidth - 1)];
            }
        }
    }

    public float getY(int row, int col) {
        if (row >= Y.length) row = Y.length - 1;
        if (col >= Y[0].length) col = Y[0].length - 1;
        return Y[row][col];
    }

    public float getCb(int row, int col) {
        if (row >= Cb.length) row = Cb.length - 1;
        if (col >= Cb[0].length) col = Cb[0].length - 1;
        return Cb[row][col];
    }

    public float getCr(int row, int col) {
        if (row >= Cr.length) row = Cr.length - 1;
        if (col >= Cr[0].length) col = Cr[0].length - 1;
        return Cr[row][col];
    }
}
