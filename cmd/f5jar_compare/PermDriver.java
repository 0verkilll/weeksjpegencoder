import net.f5.crypt.F5Random;
import net.f5.crypt.Permutation;
public class PermDriver {
    public static void main(String[] a) throws Exception {
        String pw = a[0]; int size = Integer.parseInt(a[1]); int n = Integer.parseInt(a[2]);
        F5Random r = new F5Random(pw.getBytes());
        Permutation p = new Permutation(size, r);
        StringBuilder sb = new StringBuilder("PERM ");
        for (int i=0;i<n;i++){ sb.append(p.getShuffled(i)); if(i<n-1) sb.append(","); }
        System.out.println(sb);
    }
}
