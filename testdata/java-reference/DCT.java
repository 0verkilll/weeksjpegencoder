// DCT - Standalone version based on James R. Weeks' implementation
// Copyright (C) 1998, James R. Weeks and BioElectroMech.
//
// Based on the Independent JPEG Group's work (Thomas G. Lane's Jpeg 6a library)
// and Florian Raemy's Java JPEG encoder.

/**
 * DCT - Performs forward Discrete Cosine Transform and quantization.
 */
public class DCT {
    public Object[] quantum = new Object[2];
    public double[][] divisors = new double[2][64];

    private int QUALITY;
    private static final int N = 8;

    // Standard ITU-T T.81 Annex K luminance quantization table
    private static final int[] jpegLuminanceQuantTbl = {
        16, 11, 10, 16, 24, 40, 51, 61,
        12, 12, 14, 19, 26, 58, 60, 55,
        14, 13, 16, 24, 40, 57, 69, 56,
        14, 17, 22, 29, 51, 87, 80, 62,
        18, 22, 37, 56, 68, 109, 103, 77,
        24, 35, 55, 64, 81, 104, 113, 92,
        49, 64, 78, 87, 103, 121, 120, 101,
        72, 92, 95, 98, 112, 100, 103, 99
    };

    // Standard ITU-T T.81 Annex K chrominance quantization table
    private static final int[] jpegChrominanceQuantTbl = {
        17, 18, 24, 47, 99, 99, 99, 99,
        18, 21, 26, 66, 99, 99, 99, 99,
        24, 26, 56, 99, 99, 99, 99, 99,
        47, 66, 99, 99, 99, 99, 99, 99,
        99, 99, 99, 99, 99, 99, 99, 99,
        99, 99, 99, 99, 99, 99, 99, 99,
        99, 99, 99, 99, 99, 99, 99, 99,
        99, 99, 99, 99, 99, 99, 99, 99
    };

    public DCT(int quality) {
        this.QUALITY = quality;
        initMatrix(quality);
    }

    private void initMatrix(int quality) {
        // IJG quality scaling formula
        // quality < 50: scale = 5000 / quality
        // quality >= 50: scale = 200 - 2*quality
        int scale;
        if (quality < 50) {
            scale = 5000 / quality;
        } else {
            scale = 200 - quality * 2;
        }

        // Initialize luminance quantization table
        int[] luminanceQuantTable = new int[64];
        for (int i = 0; i < 64; i++) {
            int temp = (jpegLuminanceQuantTbl[i] * scale + 50) / 100;
            if (temp <= 0) temp = 1;
            if (temp > 255) temp = 255;
            luminanceQuantTable[i] = temp;
        }
        quantum[0] = luminanceQuantTable;

        // Initialize chrominance quantization table
        int[] chrominanceQuantTable = new int[64];
        for (int i = 0; i < 64; i++) {
            int temp = (jpegChrominanceQuantTbl[i] * scale + 50) / 100;
            if (temp <= 0) temp = 1;
            if (temp > 255) temp = 255;
            chrominanceQuantTable[i] = temp;
        }
        quantum[1] = chrominanceQuantTable;

        // Initialize divisors for AAN DCT
        double[] aanScaleFactor = {
            1.0, 1.387039845, 1.306562965, 1.175875602,
            1.0, 0.785694958, 0.541196100, 0.275899379
        };

        for (int i = 0; i < 64; i++) {
            int row = i / 8;
            int col = i % 8;
            divisors[0][i] = 1.0 / (luminanceQuantTable[i] * aanScaleFactor[row] * aanScaleFactor[col] * 8.0);
            divisors[1][i] = 1.0 / (chrominanceQuantTable[i] * aanScaleFactor[row] * aanScaleFactor[col] * 8.0);
        }
    }

    /**
     * Forward DCT using the AAN (Arai, Agui, Nakajima) algorithm.
     */
    public double[][] forwardDCT(float[][] input) {
        double[][] output = new double[N][N];
        double[][] workspace = new double[N][N];

        // Constants for AAN algorithm
        final double c1 = 0.9807852804032304;  // cos(PI/16)
        final double s1 = 0.19509032201612825; // sin(PI/16)
        final double c3 = 0.8314696123025452;  // cos(3*PI/16)
        final double s3 = 0.5555702330196022;  // sin(3*PI/16)
        final double r2c6 = 0.5411961001461969; // sqrt(2)*cos(6*PI/16)
        final double r2s6 = 1.3065629648763766; // sqrt(2)*sin(6*PI/16)
        final double r2 = 1.4142135623730951;  // sqrt(2)

        // Process rows
        for (int i = 0; i < N; i++) {
            double x0 = input[i][0] + input[i][7];
            double x7 = input[i][0] - input[i][7];
            double x1 = input[i][1] + input[i][6];
            double x6 = input[i][1] - input[i][6];
            double x2 = input[i][2] + input[i][5];
            double x5 = input[i][2] - input[i][5];
            double x3 = input[i][3] + input[i][4];
            double x4 = input[i][3] - input[i][4];

            // Even part
            double t0 = x0 + x3;
            double t3 = x0 - x3;
            double t1 = x1 + x2;
            double t2 = x1 - x2;

            workspace[i][0] = t0 + t1;
            workspace[i][4] = t0 - t1;

            double z1 = (t2 + t3) * r2c6;
            workspace[i][2] = t3 + z1;
            workspace[i][6] = t3 - z1;

            // Odd part
            t0 = x4 + x5;
            t1 = x5 + x6;
            t2 = x6 + x7;

            double z5 = (t0 - t2) * r2s6;
            double z2 = t0 * r2c6 + z5;
            double z4 = t2 * r2c6 + z5;
            double z3 = t1 * r2;

            double z11 = x7 + z3;
            double z13 = x7 - z3;

            workspace[i][5] = z13 + z2;
            workspace[i][3] = z13 - z2;
            workspace[i][1] = z11 + z4;
            workspace[i][7] = z11 - z4;
        }

        // Process columns
        for (int i = 0; i < N; i++) {
            double x0 = workspace[0][i] + workspace[7][i];
            double x7 = workspace[0][i] - workspace[7][i];
            double x1 = workspace[1][i] + workspace[6][i];
            double x6 = workspace[1][i] - workspace[6][i];
            double x2 = workspace[2][i] + workspace[5][i];
            double x5 = workspace[2][i] - workspace[5][i];
            double x3 = workspace[3][i] + workspace[4][i];
            double x4 = workspace[3][i] - workspace[4][i];

            // Even part
            double t0 = x0 + x3;
            double t3 = x0 - x3;
            double t1 = x1 + x2;
            double t2 = x1 - x2;

            output[0][i] = t0 + t1;
            output[4][i] = t0 - t1;

            double z1 = (t2 + t3) * r2c6;
            output[2][i] = t3 + z1;
            output[6][i] = t3 - z1;

            // Odd part
            t0 = x4 + x5;
            t1 = x5 + x6;
            t2 = x6 + x7;

            double z5 = (t0 - t2) * r2s6;
            double z2 = t0 * r2c6 + z5;
            double z4 = t2 * r2c6 + z5;
            double z3 = t1 * r2;

            double z11 = x7 + z3;
            double z13 = x7 - z3;

            output[5][i] = z13 + z2;
            output[3][i] = z13 - z2;
            output[1][i] = z11 + z4;
            output[7][i] = z11 - z4;
        }

        return output;
    }

    /**
     * Quantize a DCT block.
     */
    public int[] quantizeBlock(double[][] input, int tableNum) {
        int[] output = new int[64];

        for (int i = 0; i < N; i++) {
            for (int j = 0; j < N; j++) {
                int idx = i * N + j;
                double temp = input[i][j] * divisors[tableNum][idx];
                output[idx] = (int) Math.round(temp);
            }
        }

        return output;
    }
}
