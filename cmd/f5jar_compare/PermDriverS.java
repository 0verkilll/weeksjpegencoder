import java.security.SecureRandom;
import java.security.MessageDigest;
// Faithful F5Random + Permutation, but FORCING SHA1PRNG (what original f5.jar used
// before Java 9 changed the default SecureRandom to NativePRNG). This is the
// deterministic reference the project validated against.
public class PermDriverS {
  static SecureRandom random; static byte[] b1 = new byte[1];
  static int getNextByte(){ random.nextBytes(b1); return b1[0]; }
  static int getNextValue(int maxValue){
    int r = getNextByte() | getNextByte()<<8 | getNextByte()<<16 | getNextByte()<<24;
    r %= maxValue; if (r<0) r+=maxValue; return r;
  }
  public static void main(String[] a) throws Exception {
    String pw=a[0]; int size=Integer.parseInt(a[1]); int n=Integer.parseInt(a[2]);
    MessageDigest md = MessageDigest.getInstance("SHA-1");
    byte[] seed = md.digest(pw.getBytes());
    random = SecureRandom.getInstance("SHA1PRNG"); random.setSeed(seed);
    int[] sh = new int[size]; for(int i=0;i<size;i++) sh[i]=i;
    int maxRandom=size;
    for(int v=0; v<size; v++){ int ri=getNextValue(maxRandom--); int t=sh[ri]; sh[ri]=sh[maxRandom]; sh[maxRandom]=t; }
    StringBuilder s=new StringBuilder("PERM ");
    for(int i=0;i<n;i++){ s.append(sh[i]); if(i<n-1) s.append(","); }
    System.out.println(s);
  }
}
