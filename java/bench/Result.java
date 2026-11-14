package bench;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;

/**
 * One run of one scenario variant on the Java client. Mirrors bench.Result. Java results carry
 * jvm_runtime and never go_runtime.
 *
 * @param scenario   scenario name.
 * @param variant    variant label.
 * @param config     scenario flags.
 * @param throughput rate of a throughput scenario, null otherwise.
 * @param latency    percentiles of a ping-pong or fixed-rate scenario, null otherwise.
 * @param fixedRate  counts and histogram of a fixed-rate scenario, null otherwise.
 * @param jvmRuntime GC totals over the measured section.
 */
record Result(
    String scenario,
    String variant,
    Config config,
    Throughput throughput,
    Latency latency,
    FixedRate fixedRate,
    JvmRuntime jvmRuntime)
{
    static final String CLIENT = "java";

    /**
     * Builds the result of a scenario without fixed-rate fields.
     */
    Result(
        final String scenario,
        final String variant,
        final Config config,
        final Throughput throughput,
        final Latency latency,
        final JvmRuntime jvmRuntime)
    {
        this(scenario, variant, config, throughput, latency, null, jvmRuntime);
    }

    Json toJson()
    {
        final Json json = new Json()
            .field("scenario", scenario)
            .field("client", CLIENT)
            .field("variant", variant)
            .field("config", config.toJson());
        if (null != throughput)
        {
            json.field("throughput", throughput.toJson());
        }
        if (null != latency)
        {
            json.field("latency", latency.toJson());
        }
        if (null != fixedRate)
        {
            json.field("fixed_rate", fixedRate.toJson());
        }
        return json.field("jvm_runtime", jvmRuntime.toJson());
    }

    /**
     * Writes the result JSON to config.output. An empty output writes to stdout.
     *
     * @throws IOException when the file does not write.
     */
    void write() throws IOException
    {
        final String text = toJson() + "\n";
        if (config.output().isEmpty())
        {
            System.out.print(text);
            System.out.flush();
        }
        else
        {
            Files.writeString(Path.of(config.output()), text, StandardCharsets.US_ASCII);
        }
    }
}
