const BASE_URL = "http://pi4.local:20128/v1/chat/completions";
const MODEL = "my-local";
const API_KEY = "aaa";

const body = {
  model: MODEL,
  stream: true,
  messages: [
    {
      role: "user",
      content: "What is the weather in Hanoi today?",
    },
  ],
  tools: [
    {
      type: "function",
      function: {
        name: "get_weather",
        description: "Get current weather for a given location",
        parameters: {
          type: "object",
          properties: {
            location: {
              type: "string",
              description: "City name, e.g. Hanoi",
            },
            unit: {
              type: "string",
              enum: ["celsius", "fahrenheit"],
            },
          },
          required: ["location"],
        },
      },
    },
  ],
};

async function main() {
  console.log("→ POST", BASE_URL);
  console.log("→ Body:", JSON.stringify(body, null, 2));

  const res = await fetch(BASE_URL, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "Authorization": `Bearer ${API_KEY}`,
    },
    body: JSON.stringify(body),
  });

  const text = await res.text();
  console.log("\n← Status:", res.status);
  let data;
  try {
    data = JSON.parse(text);
  } catch {
    console.error("SyntaxError: Failed to parse JSON. Raw response:\n", text);
    return;
  }
  console.log("← Response:", JSON.stringify(data, null, 2));
}

main().catch(console.error);
