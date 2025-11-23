# Feature Porting: CodeBreeze to Crush

> Status: Draft
> Date: 2025-11-23

## Overview

This document outlines the plan to transplant key features from **CodeBreeze** to **Crush** to enhance its capabilities, specifically focusing on context management and reasoning processes.

## Feature Comparison

| Feature | CodeBreeze | Crush | Gap Analysis |
|---------|------------|-------|--------------|
| **Auto Compact Context** | ✅ Implemented in `autoCompactCore.ts`. Automatically summarizes conversation and recovers file context when token limits are reached. | ❌ Not found in `internal/session` or `internal/agent`. Sessions grow indefinitely or rely on manual management. | **High Priority**. Essential for long-running sessions to prevent context window exhaustion and reduce costs. |
| **Thinking Process** | ✅ Implemented in `thinking.ts`. Detects keywords (e.g., "think harder") to adjust reasoning effort and token limits. | ❌ Not found. Likely uses static defaults. | **Medium Priority**. Enhances model performance on complex tasks by allowing user control over reasoning depth. |
| **Conversation Recovery** | ✅ Implemented in `conversationRecovery.ts`. Robust deserialization with legacy support and tool reconnection. | ⚠️ Basic persistence in `internal/session`. | **Low Priority**. Existing session management seems functional, but could be hardened. |

## Porting Plan

### 1. Auto Compact Context

**Goal**: Implement automatic context compression to manage token usage effectively.

**Source Logic (`CodeBreeze`)**:

- **Trigger**: Token count > Threshold (e.g., 200k context -> 160k threshold).
- **Action**:
    1. Generate summary of current conversation using LLM.
    2. Identify and read recently accessed files ("Recovered Files").
    3. Replace conversation history with:
        - Summary message.
        - Recovered file contents.
    4. Clear caches (file freshness, etc.).

**Implementation in `Crush`**:

- **Location**: `internal/agent/context_manager.go` (New) or `internal/session`.
- **Components**:
  - `CheckAutoCompact(messages []Message) bool`: Check token count.
  - `ExecuteAutoCompact(ctx context.Context, messages []Message) ([]Message, error)`: Perform compression.
  - `SummarizeConversation(messages []Message) (string, error)`: Call LLM for summary.
  - `GetRecentFiles()`: Track file access in `internal/agent` or `internal/fs`.

### 2. Thinking Process

**Goal**: Allow users to control reasoning effort via prompt keywords.

**Source Logic (`CodeBreeze`)**:

- **Keywords**: "think harder", "ultrathink", "think about it".
- **Effect**: Set `max_tokens` (e.g., 32k for "ultrathink") and `reasoning_effort` (low/medium/high).

**Implementation in `Crush`**:

- **Location**: `internal/agent/agent.go` (Message processing loop).
- **Logic**:
  - Parse user prompt before sending to LLM.
  - If keywords found, override `ModelParams`.
  - Pass `reasoning_effort` to LLM provider (if supported).

## Detailed Implementation Steps

### Phase 1: Auto Compact

1. **Token Counting**: Ensure `internal/token` (or similar) has accurate counting for messages.
2. **Summary Generation**: Add a method to `Agent` to generate summaries.
3. **File Tracking**: Ensure `Agent` tracks accessed files (already likely for tools).
4. **Integration**: In the main run loop (`agent.Run`), check token usage after each turn. If high, trigger compaction.

### Phase 2: Thinking Process

1. **Config**: Add `ThinkingConfig` to `AgentConfig`.
2. **Parser**: Implement `DetectThinkingIntent(prompt string) (effort string, tokens int)`.
3. **Integration**: Apply overrides in `agent.Chat` or `agent.Step`.

## References

- CodeBreeze: `src/utils/autoCompactCore.ts`
- CodeBreeze: `src/utils/thinking.ts`
- Crush: `internal/agent/agent.go`
