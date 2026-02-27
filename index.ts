import { GoogleGenerativeAI } from '@google/generative-ai';
import * as readline from 'readline';

async function main() {
  const genAI = new GoogleGenerativeAI(process.env.GEMINI_API_KEY);
  const model = genAI.getGenerativeModel({ model: 'gemini-1.5-flash-latest' });

  const chat = model.startChat({
    history: [],
    generationConfig: {
      maxOutputTokens: 100,
    },
  });

  const rl = readline.createInterface({
    input: process.stdin,
    output: process.stdout,
  });

  async function chatLoop() {
    rl.question('What be on yer mind, matey? ', async (userInput) => {
      const result = await chat.sendMessage(`You are a pirate. Respond to the following as a pirate: ${userInput}`);
      const response = await result.response;
      const text = response.text();
      console.log(`Captain's Log: ${text}`);
      chatLoop();
    });
  }

  chatLoop();
}

main().catch(console.error);
