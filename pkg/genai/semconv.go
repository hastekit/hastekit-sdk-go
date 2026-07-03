// Package genai holds the OpenTelemetry GenAI semantic-convention attribute
// keys and well-known values used across the SDK and the gateway. Span code is
// written inline at each call site so it is obvious what is traced; only the
// names live here so they can be changed in one place.
//
// Reference: https://opentelemetry.io/docs/specs/semconv/gen-ai/
package genai

// Attribute keys.
const (
	AttrOperationName = "gen_ai.operation.name"

	// AttrProviderName carries our internal provider identifier (e.g.
	// "bedrock", "anthropic") rather than the spec's canonicalised value
	// (e.g. "aws.bedrock"). This is deliberate — cost attribution keys
	// pricing by this exact value against model_catalogue.provider_type.
	AttrProviderName = "gen_ai.provider.name"

	AttrRequestModel       = "gen_ai.request.model"
	AttrResponseModel      = "gen_ai.response.model"
	AttrResponseID         = "gen_ai.response.id"
	AttrFinishReasons      = "gen_ai.response.finish_reasons"
	AttrUsageInputTokens   = "gen_ai.usage.input_tokens"
	AttrUsageOutputTokens  = "gen_ai.usage.output_tokens"
	AttrInputMessages      = "gen_ai.input.messages"
	AttrOutputMessages     = "gen_ai.output.messages"
	AttrSystemInstructions = "gen_ai.system_instructions"

	AttrRequestTemperature      = "gen_ai.request.temperature"
	AttrRequestTopP             = "gen_ai.request.top_p"
	AttrRequestMaxTokens        = "gen_ai.request.max_tokens"
	AttrRequestFrequencyPenalty = "gen_ai.request.frequency_penalty"
	AttrRequestPresencePenalty  = "gen_ai.request.presence_penalty"
	AttrRequestStopSequences    = "gen_ai.request.stop_sequences"
	AttrRequestSeed             = "gen_ai.request.seed"
	AttrRequestChoiceCount      = "gen_ai.request.choice.count"
	AttrRequestEncodingFormats  = "gen_ai.request.encoding_formats"

	AttrAgentName = "gen_ai.agent.name"
	AttrAgentID   = "gen_ai.agent.id"

	AttrToolName        = "gen_ai.tool.name"
	AttrToolCallID      = "gen_ai.tool.call.id"
	AttrToolDescription = "gen_ai.tool.description"
	AttrToolArguments   = "gen_ai.tool.call.arguments"
	AttrToolResult      = "gen_ai.tool.call.result"

	// AttrSessionID is the general OTel session attribute (not gen_ai.*).
	// CloudWatch GenAI Observability and AgentCore Evaluations group traces
	// into sessions by this attribute. We use the client-facing thread id as
	// the session identity, so a conversation thread = one session.
	AttrSessionID = "session.id"
)

// Supplementary hastekit attributes. The GenAI spec has no attribute for
// cached-prompt tokens, and collapses chat/responses/streaming into a single
// operation ("chat"); these keep the spec strictly honoured while the usage
// dashboard retains its breakdown.
const (
	AttrRequestType       = "hastekit.request_type"
	AttrCachedInputTokens = "hastekit.usage.cached_input_tokens"
)

// gen_ai.operation.name values (plus best-effort values for operations the
// spec does not yet cover: speech/transcription/image).
const (
	OpChat            = "chat"
	OpEmbeddings      = "embeddings"
	OpSpeech          = "speech"
	OpTranscription   = "transcription"
	OpImageGeneration = "image_generation"
	OpImageEdit       = "image_edit"
	OpExecuteTool     = "execute_tool"
	OpInvokeAgent     = "invoke_agent"
)

// hastekit.request_type values — the usage dashboard groups by these verbatim.
const (
	RequestTypeChat            = "Chat"
	RequestTypeChatStream      = "Chat (Stream)"
	RequestTypeResponses       = "Responses"
	RequestTypeResponsesStream = "Responses (Stream)"
	RequestTypeEmbeddings      = "Embeddings"
	RequestTypeSpeech          = "Speech"
	RequestTypeSpeechStream    = "Speech (Stream)"
	RequestTypeTranscription   = "Transcription"
	RequestTypeImageGeneration = "ImageGeneration"
	RequestTypeImageEdit       = "ImageEdit"
)
