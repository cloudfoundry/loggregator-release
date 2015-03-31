package org.cloudfoundry;

import com.timgroup.statsd.StatsDClient;
import com.timgroup.statsd.NonBlockingStatsDClient;

import java.util.Scanner;

public class StatsdJavaClient {

    public static final void main(String[] args) throws Exception {
        StatsDClient statsd = new NonBlockingStatsDClient("testNamespace", "localhost", 8125);
        Scanner scanner = new Scanner(System.in);
        while (scanner.hasNextLine()) {
            String line = scanner.nextLine();
            String[] inputs = line.split(" ");
            if (inputs.length < 3) {
                System.out.println("Wrong number of inputs, 3 needed at least");
                continue;
            }
            String statsdType = inputs[0];
            String name = inputs[1];
            long value = Long.parseLong(inputs[2]);
            if (statsdType.equals("count")) {
                double sampleRate = 1.0;
                if (inputs.length == 4) {
                    sampleRate = Double.parseDouble(inputs[3]);
                }
                statsd.count(name, value, sampleRate);
            } else if(statsdType.equals("gauge")) {
                statsd.recordGaugeValue(name, value);
            } else if(statsdType.equals("timing")) {
                    statsd.recordExecutionTime(name, value);
            } else {
                System.out.println("Unsupported operation: " + statsdType);
            }
        }
    }
}