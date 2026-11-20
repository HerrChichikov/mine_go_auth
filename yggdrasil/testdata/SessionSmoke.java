import com.mojang.authlib.yggdrasil.YggdrasilAuthenticationService;
import java.io.BufferedReader;
import java.io.InputStreamReader;
import java.net.Proxy;
import java.util.UUID;

class SessionSmoke {
    public static void main(String[] args) throws Exception {
        var id = UUID.fromString(args[0]);
        var service = new YggdrasilAuthenticationService(Proxy.NO_PROXY).createMinecraftSessionService();
        var token = new BufferedReader(new InputStreamReader(System.in)).readLine();
        service.joinServer(id, token, "-abc012");
        var joined = service.hasJoinedServer(args[1], "-abc012", null);
        if (joined == null || !id.equals(joined.profile().getId())) {
            throw new AssertionError("Unexpected joined profile");
        }
        var profile = service.fetchProfile(id, true);
        if (profile == null || !id.equals(profile.profile().getId())) {
            throw new AssertionError("Unexpected fetched profile");
        }
        var textures = service.getPackedTextures(profile.profile());
        if (textures == null || service.getSecurePropertyValue(textures).isEmpty()) {
            throw new AssertionError("Missing verified texture payload");
        }
        System.out.println("Injector session and signed texture passed");
    }
}
