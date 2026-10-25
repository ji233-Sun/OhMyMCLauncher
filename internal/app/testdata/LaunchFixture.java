// A real JVM fixture, not a Minecraft client. Used only by service integration tests.
public final class LaunchFixture {
    public static void main(String[] args) {
        System.out.println("OHMYMC_JAVA_OK");
        System.out.println("property=" + System.getProperty("ohmymc.fixture"));
        System.out.println("cwd=" + System.getProperty("user.dir"));
        for (int i = 0; i < args.length; i++) {
            System.out.println("arg" + i + "=" + args[i]);
        }
        System.err.println("OHMYMC_STDERR_OK");
    }
}
