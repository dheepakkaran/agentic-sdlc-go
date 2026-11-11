window.METRICS = {
  "ai_authored_merged": 0,
  "by_model": {},
  "by_role": {},
  "cache_read_share": 0,
  "generated_at": "2026-11-11T18:42:46Z",
  "harness_total": {
    "calls": 0,
    "input_tokens": 0,
    "output_tokens": 0,
    "cache_read_tokens": 0,
    "cache_write_tokens": 0,
    "cost_usd": 0,
    "seconds": 0
  },
  "issues": null,
  "merged_prs": 0,
  "naive_total": {
    "calls": 0,
    "input_tokens": 0,
    "output_tokens": 0,
    "cache_read_tokens": 0,
    "cache_write_tokens": 0,
    "cost_usd": 0,
    "seconds": 0
  },
  "pr_open_to_merge_h": [],
  "prices_per_mtok": {
    "claude-haiku-5-5": {
      "In": 0.1,
      "Out": 0.5,
      "CacheRead": 0.01,
      "CacheWrite": 0.125
    },
    "claude-opus-5-5": {
      "In": 4,
      "Out": 20,
      "CacheRead": 0.2,
      "CacheWrite": 5
    },
    "claude-sonnet-5-5": {
      "In": 2,
      "Out": 10,
      "CacheRead": 0.1,
      "CacheWrite": 2.5
    }
  },
  "prs": [],
  "routes": {
    "analyst": {
      "Model": "claude-haiku-5-5",
      "Effort": "low",
      "MaxTokens": 4000,
      "Escalate": "claude-sonnet-5-5"
    },
    "architect": {
      "Model": "claude-opus-5-5",
      "Effort": "high",
      "MaxTokens": 16000,
      "Escalate": ""
    },
    "developer": {
      "Model": "claude-sonnet-5-5",
      "Effort": "medium",
      "MaxTokens": 32000,
      "Escalate": "claude-opus-5-5"
    },
    "docs": {
      "Model": "claude-haiku-5-5",
      "Effort": "low",
      "MaxTokens": 4000,
      "Escalate": "claude-sonnet-5-5"
    },
    "naive": {
      "Model": "claude-opus-5-5",
      "Effort": "high",
      "MaxTokens": 64000,
      "Escalate": ""
    },
    "qa": {
      "Model": "claude-opus-5-5",
      "Effort": "high",
      "MaxTokens": 16000,
      "Escalate": ""
    }
  }
};
