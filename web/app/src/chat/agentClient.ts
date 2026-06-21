import { createClient } from "@connectrpc/connect";
import { AgentService } from "../gen/internal/agent/v1/agent_pb";
import { transport } from "../transport";

// Server-streaming client; agentClient.ask({text}) returns an AsyncIterable<AskEvent>.
export const agentClient = createClient(AgentService, transport);
