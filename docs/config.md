| Field    | Type   | Description                                                                                                                                                                                                                       | Default                 | Required  |
| :------- | :----- | :-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | :---------------------- | :-------- |
| timezone | string | The timezone of the app. e.g. `Asia/Shanghai`.                                                                                                                                                                                    | server's local timezone | No        |
| log      | object | The log config. See **Log Configuration** section below.                                                                                                                                                                          | (see fields)            | No        |
| api      | object | The API config. See **API Configuration** section below.                                                                                                                                                                          | (see fields)            | No        |
| llms     | list   | The LLMs config. Refered by other config sections. See **LLM Configuration** section below.                                                                                                                                       | `[]`                    | Yes (>=1) |
| scrape   | object | The scrape config. See **Scrape Configuration** section below.                                                                                                                                                                    | (see fields)            | No        |
| storage  | object | The storage config. See **Storage Configuration** section below.                                                                                                                                                                  | (see fields)            | No        |
| scheduls | object | The scheduls config for monitoring feeds (aka monitoring rules). See **Scheduls Configuration** section below.                                                                                                                    | (see fields)            | No        |
| notify   | object | The notify config. It receives results from scheduls, groups them via route config, and sends to receivers via channels. See **Notify Configuration**, **Notify Route**, **Notify Receiver**, **Notify Channels** sections below. | (see fields)            | Yes       |

### Log Configuration (`log`)

| Field       | Type   | Description                                         | Default | Required |
| :---------- | :----- | :-------------------------------------------------- | :------ | :------- |
| `log.level` | string | Log level, one of `debug`, `info`, `warn`, `error`. | `info`  | No       |

**API Configuration (`api`)**

| Field              | Type   | Description                                                                                                         | Default                       | Required                               |
| :----------------- | :----- | :------------------------------------------------------------------------------------------------------------------ | :---------------------------- | :------------------------------------- |
| `api.http`         | object | The HTTP API config.                                                                                                | (see fields)                  | No                                     |
| `api.http.address` | string | The address (`[host]:port`) of the HTTP API. e.g. `0.0.0.0:1300`. Cannot be changed after the app is running.       | `:1300`                       | No                                     |
| `api.mcp`          | object | The MCP API config.                                                                                                 | (see fields)                  | No                                     |
| `api.mcp.address`  | string | The address (`[host]:port`) of the MCP API. e.g. `0.0.0.0:1301`. Cannot be changed after the app is running.        | `:1301`                       | No                                     |
| `api.llm`          | string | The LLM name for summarizing feeds. e.g. `my-favorite-gemini-king`. Refers to an LLM defined in the `llms` section. | default LLM in `llms` section | Yes (if summarization feature is used) |

### LLM Configuration (`llms[]`)

This section defines a list of available Large Language Models. At least one LLM configuration is required.

| Field                    | Type    | Description                                                                                                                                                                                                                                   | Default                     | Required                                                       |
| :----------------------- | :------ | :-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | :-------------------------- | :------------------------------------------------------------- |
| `llms[].name`            | string  | The name (or 'id') of the LLM. e.g. `my-favorite-gemini-king`. Used to refer to this LLM in other sections (`api.llm`, `storage.feed.embedding_llm`, etc.).                                                                                   |                             | Yes                                                            |
| `llms[].default`         | bool    | Whether this LLM is the default LLM. Only one LLM can be the default.                                                                                                                                                                         | `false`                     | No (but one must be `true` if default behavior is relied upon) |
| `llms[].provider`        | string  | The provider of the LLM, one of `openai`, `openrouter`, `deepseek`, `gemini`, `volc`, `siliconflow`. e.g. `openai`.                                                                                                                           |                             | Yes                                                            |
| `llms[].endpoint`        | string  | The custom endpoint of the LLM. e.g. `https://api.openai.com/v1`.                                                                                                                                                                             | (provider specific default) | No                                                             |
| `llms[].api_key`         | string  | The API key of the LLM.                                                                                                                                                                                                                       |                             | Yes                                                            |
| `llms[].model`           | string  | The model of the LLM. e.g. `gpt-4o-mini`. Cannot be empty if used for generation tasks (like summarization). Cannot be empty with `embedding_model` at same time if this LLM is used.                                                         |                             | Conditionally Yes                                              |
| `llms[].embedding_model` | string  | The embedding model of the LLM. e.g. `text-embedding-3-small`. Cannot be empty if used for embedding. Cannot be empty with `model` at same time if this LLM is used. **NOTE:** Do not modify after initial use; add a new LLM config instead. |                             | Conditionally Yes                                              |
| `llms[].temperature`     | float32 | The temperature (0-2) of the LLM.                                                                                                                                                                                                             | `0.0`                       | No                                                             |

### Scrape Configuration (`scrape`)

| Field                    | Type            | Description                                                                                                                                                      | Default | Required                          |
| :----------------------- | :-------------- | :--------------------------------------------------------------------------------------------------------------------------------------------------------------- | :------ | :-------------------------------- |
| `scrape.past`            | duration        | The lookback time window for scraping feeds. e.g. `1h` means only scrape feeds in the past 1 hour.                                                               | `3d`    | No                                |
| `scrape.interval`        | duration        | How often to scrape each source (global default). e.g. `1h`.                                                                                                     | `1h`    | No                                |
| `scrape.rsshub_endpoint` | string          | The endpoint of the RSSHub. You can deploy your own or use a public one (see [RSSHub Docs](https://docs.rsshub.app/guide/instances)). e.g. `https://rsshub.app`. |         | Yes (if `rsshub_route_path` used) |
| `scrape.sources`         | list of objects | The sources for scraping feeds. See **Scrape Source Configuration** below.                                                                                       | `[]`    | Yes (at least one)                |

### Scrape Source Configuration (`scrape.sources[]`)

Describes each source to be scraped.

| Field                       | Type              | Description                                                                                                                            | Default         | Required                    |
| :-------------------------- | :---------------- | :------------------------------------------------------------------------------------------------------------------------------------- | :-------------- | :-------------------------- |
| `scrape.sources[].interval` | duration          | How often to scrape this specific source. Overrides the global `scrape.interval`.                                                      | global interval | No                          |
| `scrape.sources[].name`     | string            | The name of the source. Used for labeling feeds.                                                                                       |                 | Yes                         |
| `scrape.sources[].labels`   | map[string]string | Additional key-value labels to add to feeds from this source.                                                                          | `{}`            | No                          |
| `scrape.sources[].rss`      | object            | The RSS config for this source. See **Scrape Source RSS Configuration** below. Only one source type (e.g., RSS) can be set per source. | `nil`           | Yes (if source type is RSS) |

### Scrape Source RSS Configuration (`scrape.sources[].rss`)

| Field                                    | Type   | Description                                                                                                                           | Default | Required                                              |
| :--------------------------------------- | :----- | :------------------------------------------------------------------------------------------------------------------------------------ | :------ | :---------------------------------------------------- |
| `scrape.sources[].rss.url`               | string | The full URL of the RSS feed. e.g. `http://localhost:1200/github/trending/daily/any`. Cannot be set if `rsshub_route_path` is set.    |         | Yes (unless `rsshub_route_path` is set)               |
| `scrape.sources[].rss.rsshub_route_path` | string | The RSSHub route path. e.g. `github/trending/daily/any`. Will be joined with `scrape.rsshub_endpoint`. Cannot be set if `url` is set. |         | Yes (unless `url` is set, requires `rsshub_endpoint`) |

### Storage Configuration (`storage`)

| Field          | Type   | Description                                                                      | Default      | Required |
| :------------- | :----- | :------------------------------------------------------------------------------- | :----------- | :------- |
| `storage.dir`  | string | The base directory for all storages. Cannot be changed after the app is running. | `./data`     | No       |
| `storage.feed` | object | The feed storage config. See **Feed Storage Configuration** below.               | (see fields) | No       |

### Feed Storage Configuration (`storage.feed`)

| Field                         | Type            | Description                                                                                                                                                                             | Default                       | Required |
| :---------------------------- | :-------------- | :-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | :---------------------------- | :------- |
| `storage.feed.rewrites`       | list of objects | How to process each feed before storing it. Inspired by Prometheus relabeling. See **Rewrite Rule Configuration** below.                                                                | `[]`                          | No       |
| `storage.feed.flush_interval` | duration        | How often to flush feed storage to the database. Higher value risks data loss but improves performance.                                                                                 | `200ms`                       | No       |
| `storage.feed.embedding_llm`  | string          | The name of the LLM (from `llms` section) used for embedding feeds. Affects semantic search accuracy. **NOTE:** If changing, keep the old LLM config defined as past data relies on it. | default LLM in `llms` section | No       |
| `storage.feed.retention`      | duration        | How long to keep a feed.                                                                                                                                                                | `8d`                          | No       |
| `storage.feed.block_duration` | duration        | How long to keep each time-based feed storage block (similar to Prometheus TSDB Block).                                                                                                 | `25h`                         | No       |

### Rewrite Rule Configuration (`storage.feed.rewrites[]`)

Defines rules to process feeds before storage. Rules are applied in order.

| Field                                    | Type   | Description                                                                                                                                                                                                             | Default                  | Required                                      |
| :--------------------------------------- | :----- | :---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | :----------------------- | :-------------------------------------------- |
| `...rewrites[].source_label`             | string | The feed label to use as the source text for transformation. Default labels: `type`, `source`, `title`, `link`, `pub_time`, `content`.                                                                                  | `content`                | No                                            |
| `...rewrites[].skip_too_short_threshold` | *int   | If set, feeds where the `source_label` text length is below this threshold are skipped by this rule (processing continues with the next rule or feed storage if no more rules). Helps filter short/uninformative feeds. | `300`                    | No                                            |
| `...rewrites[].transform`                | object | Configures how to transform the `source_label` text. See **Rewrite Rule Transform Configuration** below. If unset, the `source_label` text is used directly for matching.                                               | `nil`                    | No                                            |
| `...rewrites[].match`                    | string | A simple string to match against the (transformed) text. Cannot be set with `match_re`.                                                                                                                                 |                          | No (use `match` or `match_re`)                |
| `...rewrites[].match_re`                 | string | A regular expression to match against the (transformed) text.                                                                                                                                                           | `.*` (matches all)       | No (use `match` or `match_re`)                |
| `...rewrites[].action`                   | string | Action to perform if matched: `create_or_update_label` (adds/updates a label with the matched/transformed text), `drop_feed` (discards the feed entirely).                                                              | `create_or_update_label` | No                                            |
| `...rewrites[].label`                    | string | The feed label name to create or update.                                                                                                                                                                                |                          | Yes (if `action` is `create_or_update_label`) |

### Rewrite Rule Transform Configuration (`storage.feed.rewrites[].transform`)

| Field                  | Type   | Description                                                                                                 | Default | Required |
| :--------------------- | :----- | :---------------------------------------------------------------------------------------------------------- | :------ | :------- |
| `...transform.to_text` | object | Transform the source text to text using an LLM. See **Rewrite Rule Transform To Text Configuration** below. | `nil`   | No       |

### Rewrite Rule Transform To Text Configuration (`storage.feed.rewrites[].transform.to_text`)

| Field               | Type   | Description                                                                                                                                                                                                                                       | Default                       | Required |
| :------------------ | :----- | :------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | :---------------------------- | :------- |
| `...to_text.type`   | string | The type of the transform. It can be one of `prompt`, `scrape`. Default is `prompt`. For `scrape`, the source text will be as the url to crawl the page, and the page will be converted to markdown.                                              | `prompt`                      | No       |
| `...to_text.llm`    | string | The name of the LLM (from `llms` section) to use for transformation.                                                                                                                                                                              | default LLM in `llms` section | No       |
| `...to_text.prompt` | string | The prompt used for transformation. The source text is injected. Go template syntax can refer to built-in prompts: `{{ .summary }}`, `{{ .category }}`, `{{ .tags }}`, `{{ .score }}`, `{{ .comment_confucius }}`, `{{ .summary_html_snippet }}`. |                               | Yes      |

### Scheduls Configuration (`scheduls`)

Defines rules for querying and monitoring feeds.

| Field            | Type            | Description                                                                                                                                        | Default | Required |
| :--------------- | :-------------- | :------------------------------------------------------------------------------------------------------------------------------------------------- | :------ | :------- |
| `scheduls.rules` | list of objects | The rules for scheduling feeds. Each rule's result (matched feeds) is sent to the notify route. See **Scheduls Rule Configuration** section below. | `[]`    | No       |

### Scheduls Rule Configuration (`scheduls.rules[]`)

| Field                             | Type            | Description                                                                                                                                                                                  | Default | Required                                 |
| :-------------------------------- | :-------------- | :------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | :------ | :--------------------------------------- |
| `scheduls.rules[].name`           | string          | The name of the rule.                                                                                                                                                                        |         | Yes                                      |
| `scheduls.rules[].query`          | string          | The semantic query to find relevant feeds. Optional.                                                                                                                                         |         | No                                       |
| `scheduls.rules[].threshold`      | float32         | Relevance score threshold (0-1) to filter semantic query results. Only works if `query` is set.                                                                                              | `0.6`   | No                                       |
| `scheduls.rules[].label_filters`  | list of strings | Filters based on feed labels (exact match or non-match). e.g. `["category=tech", "source!=github"]`.                                                                                         | `[]`    | No                                       |
| `scheduls.rules[].every_day`      | string          | Query range relative to the end of each day. Format: `start~end` (HH:MM). e.g., `00:00~23:59` (today), `-22:00~07:00` (yesterday 22:00 to today 07:00). Cannot be set with `watch_interval`. |         | No (use `every_day` or `watch_interval`) |
| `scheduls.rules[].watch_interval` | duration        | How often to run the query. e.g. `10m`. Cannot be set with `every_day`.                                                                                                                      | `10m`   | No (use `every_day` or `watch_interval`) |

### Notify Configuration (`notify`)

| Field              | Type            | Description                                                                                                    | Default      | Required                |
| :----------------- | :-------------- | :------------------------------------------------------------------------------------------------------------- | :----------- | :---------------------- |
| `notify.route`     | object          | The main notify routing configuration. See **Notify Route Configuration** below.                               | (see fields) | Yes                     |
| `notify.receivers` | list of objects | Defines the notification receivers (e.g., email addresses). See **Notify Receiver Configuration** below.       | `[]`         | Yes (at least one)      |
| `notify.channels`  | object          | Configures the notification channels (e.g., email SMTP settings). See **Notify Channels Configuration** below. | (see fields) | Yes (if using channels) |

### Notify Route Configuration (`notify.route` and `notify.route.sub_routes[]`)

This structure can be nested using `sub_routes`. A feed is matched against sub-routes first; if no sub-route matches, the parent route's configuration applies.

| Field                              | Type            | Description                                                                                                                                                                 | Default                       | Required             |
| :--------------------------------- | :-------------- | :-------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | :---------------------------- | :------------------- |
| `...matchers` (only in sub-routes) | list of strings | Label matchers to determine if a feed belongs to this sub-route. e.g. `["category=tech", "source!=github"]`.                                                                | `[]`                          | Yes (for sub-routes) |
| `...receivers`                     | list of strings | Names of the receivers (defined in `notify.receivers`) to send notifications for feeds matching this route.                                                                 | `[]`                          | Yes (at least one)   |
| `...group_by`                      | list of strings | Labels to group feeds by before sending notifications. Each group results in a separate notification. e.g., `["source", "category"]`.                                       | `[]`                          | Yes (at least one)   |
| `...source_label`                  | string          | The source label to extract the content from each feed, and summarize them. Default are all labels. It is very recommended to set it to 'summary' to reduce context length. | all labels                    | No                   |
| `...summary_prompt`                | string          | The prompt to summarize the feeds of each group.                                                                                                                            |                               | No                   |
| `...llm`                           | string          | The LLM name to use. Default is the default LLM in `llms` section. A large context length LLM is recommended.                                                               | default LLM in `llms` section | No                   |
| `...compress_by_related_threshold` | *float32        | If set, compresses highly similar feeds (based on semantic relatedness) within a group, sending only one representative. Threshold (0-1). Higher means more similar.        | `0.85`                        | No                   |
| `...sub_routes`                    | list of objects | Nested routes. Allows defining more specific routing rules. Each object follows the **Notify Route Configuration**.                                                         | `[]`                          | No                   |

### Notify Receiver Configuration (`notify.receivers[]`)

Defines *who* receives notifications.

| Field                      | Type   | Description                                      | Default | Required             |
| :------------------------- | :----- | :----------------------------------------------- | :------ | :------------------- |
| `notify.receivers[].name`  | string | The unique name of the receiver. Used in routes. |         | Yes                  |
| `notify.receivers[].email` | string | The email address of the receiver.               |         | Yes (if using email) |

### Notify Channels Configuration (`notify.channels`)

Configures *how* notifications are sent.

| Field                   | Type   | Description                                                                        | Default | Required             |
| :---------------------- | :----- | :--------------------------------------------------------------------------------- | :------ | :------------------- |
| `notify.channels.email` | object | The global email channel config. See **Notify Channel Email Configuration** below. | `nil`   | Yes (if using email) |

### Notify Channel Email Configuration (`notify.channels.email`)

| Field                                 | Type   | Description                                                                                                                                                                                          | Default          | Required |
| :------------------------------------ | :----- | :--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | :--------------- | :------- |
| `...email.smtp_endpoint`              | string | The SMTP server endpoint. e.g. `smtp.gmail.com:587`.                                                                                                                                                 |                  | Yes      |
| `...email.from`                       | string | The sender email address.                                                                                                                                                                            |                  | Yes      |
| `...email.password`                   | string | The application password for the sender email. (For Gmail, see [Google App Passwords](https://support.google.com/mail/answer/185833)).                                                               |                  | Yes      |
| `...email.feed_markdown_template`     | string | Markdown template for formatting each feed in the email body. Default renders the feed content. Cannot be set with `feed_html_snippet_template`. Available template variables depend on feed labels. | `{{ .content }}` | No       |
| `...email.feed_html_snippet_template` | string | HTML snippet template for formatting each feed. Cannot be set with `feed_markdown_template`. Available template variables depend on feed labels.                                                     |                  | No       |