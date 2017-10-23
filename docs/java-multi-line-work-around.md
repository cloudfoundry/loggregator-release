### Multi-line Java message workaround

Having log lines out of order can be painful when monitoring Java stack traces. 
There is a work around involving modifing the Java log output to have your app
reformat your stacktrace messages so any newline characters are replaced by a 
token; and then have your log parsing code replace that token with newline 
characters again. The following example is for the Java Logback library and ELK,
but you may be able to use the same strategy for other Java log libraries and 
log aggregators.

With the Java Logback library you do this by adding `%replace(%xException){'\n','\u2028'}%nopex` 
to your logging config , and then use the following logstash conf.
```
# Replace the unicode newline character \u2028 with \n, which Kibana will display as a new line.
    mutate {
      gsub => [ "[@message]", '\u2028', "
"]
# ^^^ Seems that passing a string with an actual newline in it is the only way to make gsub work
    }
```
to replace the token with a regular newline again so it displays "properly" in Kibana.
