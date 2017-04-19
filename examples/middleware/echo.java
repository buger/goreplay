import java.io.BufferedReader;
import java.io.IOException;
import java.io.InputStreamReader;

import org.apache.commons.codec.DecoderException;
import org.apache.commons.codec.binary.Hex;


class Echo {
    public static String decodeHexString(String s) throws DecoderException {
        return new String(Hex.decodeHex(s.toCharArray()));
    }

    public static String encodeHexString(String s) {
        return new String(Hex.encodeHex(s.getBytes()));
    }

    public static String transformHTTPMessage(String req) {
        // do actual transformations here
        return req;
    }

    public static void main(String[] args) throws DecoderException {
        if(args != null){
            for(String arg : args){
                System.out.println(arg);
            }

        }

        BufferedReader stdin = new BufferedReader(new InputStreamReader(
                                                                        System.in));
        String line = null;

        try {
            while ((line = stdin.readLine()) != null) {
                String decodedLine = decodeHexString(line);

                String transformedLine = transformHTTPMessage(decodedLine);

                String encodedLine = encodeHexString(transformedLine);
                System.out.println(encodedLine);

            }
        } catch (IOException e) {
        }
    }
}
