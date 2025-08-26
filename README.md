<p align="center">
  <code>gohenry</code> is a Claude-powered Matrix chatbot.
</p>

#

### Main Features

- Powered by Claude
- Matrix protocol integration
- Configurable context window
- 1:1 and group chats

#

> [!IMPORTANT]
> `gohenry` is pre-alpha software.

> [!CAUTION]
> `gohenry` is part of a [vibe coding] project.

***

### Usage

In direct messages, Henry will respond to all messages from users. In group chats, Henry only responds when directly addressed with a mention:

```
henry, what's the answer to the ultimate question of life, the universe and everything?
```

To configure `gohenry` use environment variables.

#### Required Environment Variables

- `HENRY_MATRIX_HOMESERVER`: URL of the Matrix homeserver (e.g., "https://matrix.org")
- `HENRY_MATRIX_USER_ID`: Matrix user ID for the bot (e.g., "@henry:matrix.org")
- `HENRY_CLAUDE_API_KEY`: Claude API key for authentication

#### Authentication (one of the following is required)

- `HENRY_MATRIX_ACCESS_TOKEN`: Pre-authenticated access token for Matrix
- `HENRY_MATRIX_PASSWORD`: Password for the Matrix account (if access token isn't provided)

#### Optional Environment Variables

- `HENRY_CONTEXT_MESSAGE_COUNT`: Number of previous messages to include as context (default: 10)
- `HENRY_ALLOWED_DOMAIN`: Domain to restrict responses to (e.g., "matrix.org")

#### Running the Bot

```bash
./gohenry
```
#### Debug Mode

```bash
./gohenry debug
```

### License

The package may be used under the terms of the ISC License a copy of
which may be found in the file [LICENSE].

Unless you explicitly state otherwise, any contribution submitted for inclusion
in the work by you shall be licensed as above, without any additional terms or
conditions.

[LICENSE]: https://github.com/huhndev/gohenry/blob/master/LICENSE
[vibe coding]: https://en.wikipedia.org/wiki/Vibe_coding
