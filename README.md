# Irelia - Automated Interview Service

Irelia is a service that manages conversations between users and [TTS & Lip Sync](https://github.com/ginganotnigg/Karma) models for interview simulations. It uses gRPC and HTTP for communication and integrates with [Darius service](https://github.com/phuxuan2k3/Darius) for dynamic question generation.

## Features

- Start interview sessions with customizable contexts (field, language, gender)
- Generate appropriate voice reader IDs for TTS and Lip Sync models
- Provide prepared interview questions sequentially
- Collect user responses and generate contextual follow-up questions
- Support for skipping introductory questions
- Manage the interview flow with proper question sequencing

## License

[MIT License](LICENSE)