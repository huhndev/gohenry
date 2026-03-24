<p align="center">
  <code>gohenry</code> is a <a href="https://www.anthropic.com/claude">Claude</a>-powered <a href="https://matrix.org">Matrix</a> chatbot.
</p>

---

### Usage

```
HENRY_MATRIX_HOMESERVER=https://matrix.example.com HENRY_MATRIX_USER_ID=@henry:example.com HENRY_CLAUDE_API_KEY=sk-... HENRY_MATRIX_PASSWORD=... gohenry
```

| Variable | Required | Default | Description |
|---|---|---|---|
| `HENRY_MATRIX_HOMESERVER` | yes | | Base URL of the Matrix homeserver |
| `HENRY_MATRIX_USER_ID` | yes | | Matrix user ID for the bot |
| `HENRY_CLAUDE_API_KEY` | yes | | Claude API key |
| `HENRY_MATRIX_ACCESS_TOKEN` | one of | | Pre-authenticated access token |
| `HENRY_MATRIX_PASSWORD` | one of | | Matrix account password |
| `HENRY_CONTEXT_MESSAGE_COUNT` | no | `10` | Number of previous messages to include as context |
| `HENRY_ALLOWED_DOMAIN` | no | `henhouse.im` | Domain to restrict responses to |

In direct messages, Henry responds to all messages. In group chats, Henry only responds when addressed by name.

### License

The package may be used under the terms of the ISC License a copy of which may be found in the file [LICENSE](LICENSE).

Unless you explicitly state otherwise, any contribution submitted for inclusion in the work by you shall be licensed as above, without any additional terms or conditions.

---

Built with AI assistance.
