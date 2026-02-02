# Gemini

## Non-Streaming GenerateContent
```json
{
	"candidates": [
		{
			"content": {
				"parts": [
					{
						"text": "Hey there! How can I help you today?"
					}
				],
				"role": "model"
			},
			"finishReason": "STOP",
			"index": 0
		}
	],
	"usageMetadata": {
		"promptTokenCount": 8,
		"candidatesTokenCount": 10,
		"totalTokenCount": 111,
		"promptTokensDetails": [
			{
				"modality": "TEXT",
				"tokenCount": 8
			}
		],
		"thoughtsTokenCount": 93
	},
	"modelVersion": "gemini-2.5-flash",
	"responseId": "B_4saZqCLv3w4-EP8ta6gQ8"
}
```

## Streaming Generate Content
```json
[
	{
		"candidates": [
			{
				"content": {
					"parts": [
						{
							"text": "Hey there! How can I help you today?"
						}
					],
					"role": "model"
				},
				"finishReason": "STOP",
				"index": 0
			}
		],
		"usageMetadata": {
			"promptTokenCount": 8,
			"candidatesTokenCount": 10,
			"totalTokenCount": 83,
			"promptTokensDetails": [
				{
					"modality": "TEXT",
					"tokenCount": 8
				}
			],
			"thoughtsTokenCount": 65
		},
		"modelVersion": "gemini-2.5-flash",
		"responseId": "Wv4saaKqAtKAg8UP1_-V4AM"
	}
]
```