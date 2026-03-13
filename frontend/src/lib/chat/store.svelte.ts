import { chatBaseUrl } from '$lib/api';

export type MessageRole = 'user' | 'assistant' | 'system';

export interface Message {
    id: string;
    role: MessageRole;
    content: string;
    timestamp: Date;
}

export type ChatEventType = 
    | 'message.delta' 
    | 'message.completed' 
    | 'tool.started' 
    | 'tool.completed' 
    | 'structured.result' 
    | 'error';

export interface ChatEvent {
    event: ChatEventType;
    data: any;
}

// Svelte 5 Chat Store using Runes
export class ChatSession {
    sessionId = $state<string | null>(null);
    messages = $state<Message[]>([]);
    structuredResults = $state<any[]>([]);
    isStreaming = $state(false);
    activeTool = $state<string | null>(null);
    error = $state<string | null>(null);

    constructor() {}

    async createSession(type: string, context: Record<string, string>) {
        try {
            const response = await fetch(`${chatBaseUrl}/v1/chat/sessions`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ type, context })
            });
            const data = await response.json();
            this.sessionId = data.sessionId;
            this.messages = [];
            this.structuredResults = [];
            this.error = null;
        } catch (err: any) {
            this.error = 'Failed to create session: ' + err.message;
        }
    }

    async sendMessage(content: string) {
        if (!this.sessionId) return;

        this.structuredResults = []; // Clear old results
        const userMsg: Message = {
            id: 'msg_' + Date.now(),
            role: 'user',
            content,
            timestamp: new Date()
        };

        this.messages.push(userMsg);
        this.error = null;

        try {
            await fetch(`${chatBaseUrl}/v1/chat/sessions/${this.sessionId}/messages`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ message: content })
            });

            this.startStreaming();
        } catch (err: any) {
            this.error = 'Failed to send message: ' + err.message;
        }
    }

    private startStreaming() {
        if (!this.sessionId) return;

        this.isStreaming = true;
        const eventSource = new EventSource(`${chatBaseUrl}/v1/chat/sessions/${this.sessionId}/stream`);

        let assistantMsgContent = '';
        const assistantMsgId = 'msg_ast_' + Date.now();

        eventSource.addEventListener('message.delta', (e: any) => {
            const data = JSON.parse(e.data);
            assistantMsgContent += data.delta;
            
            // In Svelte 5, we should find and update the object in the reactive array
            const msg = this.messages.find(m => m.id === assistantMsgId);
            if (msg) {
                msg.content = assistantMsgContent;
            } else {
                this.messages.push({
                    id: assistantMsgId,
                    role: 'assistant',
                    content: assistantMsgContent,
                    timestamp: new Date()
                });
            }
        });

        eventSource.addEventListener('tool.started', (e: any) => {
            const data = JSON.parse(e.data);
            this.activeTool = data.tool;
        });

        eventSource.addEventListener('tool.completed', () => {
            this.activeTool = null;
        });

        eventSource.addEventListener('structured.result', (e: any) => {
            const data = JSON.parse(e.data);
            this.structuredResults.push(data);
        });

        eventSource.addEventListener('message.completed', () => {
            this.isStreaming = false;
            eventSource.close();
        });

        eventSource.addEventListener('error', (e: any) => {
            this.error = 'Streaming error';
            this.isStreaming = false;
            eventSource.close();
        });
    }

    async commitAction(action: string, payload: any) {
        try {
            const response = await fetch(`${chatBaseUrl}/v1/chat/actions/commit`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ action, payload })
            });
            if (response.ok) {
                // Remove the action from results
                this.structuredResults = this.structuredResults.filter(r => r.payload !== payload);
                return true;
            }
        } catch (err: any) {
            this.error = 'Failed to commit action: ' + err.message;
        }
        return false;
    }
}

export const chatSession = new ChatSession();
