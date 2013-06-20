# PRISM

### Logging in the Clouds

###### [Tracker]() | [Mailing List]()

PRISM is the user application logging subsystem for Cloud Foundry.

### Features

PRISM allows users to:

1. Tail their application logs.
1. Dump a recent set of application logs (where recent is on the order of an hour).
1. Continually drain their application logs to 3rd party log archive and analysis services.

### Constraints

1. A PRISM outage must not affect the running application.
1. PRISM gathers and stores logs in a best-effort manner.  While undesirable, losing the current buffer of application logs is acceptable.
1. As much as possible, PRISM should be disconnected from the rest of Cloud Foundry.  Ideally, it's deployable outside of Cloud Foundry, entirely.
1. The 3rd party drain API should mimic Heroku's in order to reduce integration effort for our partners.

### Architecture

PRISM is composed of:

* **Sources**: Logging agents that run on the Cloud Foundry components.  They forward logs to:
* **PRISM Catcher**: Responsible for gathering logs from the **sources**, and storing in the temporary buffers.
* **PRISM Server**: Accepts connections from the `cf` CLI, allowing users to access their logs.
* **PRISM Drainer**: Implements the Heroku Drain API for 3rd party partners.

![PRISM Diagram](docs/prism.png)
