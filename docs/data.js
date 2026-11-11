window.METRICS = {
  "ai_authored_merged": 3,
  "all_calls_total": {
    "calls": 77,
    "input_tokens": 137233,
    "output_tokens": 102933,
    "cache_read_tokens": 11374,
    "cache_write_tokens": 150125,
    "cost_usd": 2.133510625,
    "seconds": 764.8814353169997
  },
  "all_opus_total": {
    "calls": 5,
    "input_tokens": 14205,
    "output_tokens": 11372,
    "cache_read_tokens": 0,
    "cache_write_tokens": 4672,
    "cost_usd": 0.30762,
    "seconds": 105.212745563
  },
  "by_model": {
    "claude-haiku-5-5": {
      "calls": 7,
      "input_tokens": 9669,
      "output_tokens": 3782,
      "cache_read_tokens": 0,
      "cache_write_tokens": 0,
      "cost_usd": 0.0028579,
      "seconds": 24.024442163000003
    },
    "claude-opus-5-5": {
      "calls": 6,
      "input_tokens": 16379,
      "output_tokens": 7080,
      "cache_read_tokens": 0,
      "cache_write_tokens": 9971,
      "cost_usd": 0.256971,
      "seconds": 73.24073047799999
    },
    "claude-sonnet-5-5": {
      "calls": 7,
      "input_tokens": 26603,
      "output_tokens": 12793,
      "cache_read_tokens": 3335,
      "cache_write_tokens": 27215,
      "cost_usd": 0.24950699999999998,
      "seconds": 80.06901582600001
    }
  },
  "by_role": {
    "analyst": {
      "calls": 4,
      "input_tokens": 2441,
      "output_tokens": 3612,
      "cache_read_tokens": 0,
      "cache_write_tokens": 0,
      "cost_usd": 0.0170829,
      "seconds": 24.523656447
    },
    "architect": {
      "calls": 3,
      "input_tokens": 4247,
      "output_tokens": 3797,
      "cache_read_tokens": 0,
      "cache_write_tokens": 5122,
      "cost_usd": 0.11853799999999999,
      "seconds": 38.404855034
    },
    "developer": {
      "calls": 5,
      "input_tokens": 24078,
      "output_tokens": 10603,
      "cache_read_tokens": 3335,
      "cache_write_tokens": 27215,
      "cost_usd": 0.222557,
      "seconds": 62.02416022800001
    },
    "docs": {
      "calls": 5,
      "input_tokens": 9753,
      "output_tokens": 2360,
      "cache_read_tokens": 0,
      "cache_write_tokens": 0,
      "cost_usd": 0.012725,
      "seconds": 17.545641314
    },
    "qa": {
      "calls": 3,
      "input_tokens": 12132,
      "output_tokens": 3283,
      "cache_read_tokens": 0,
      "cache_write_tokens": 4849,
      "cost_usd": 0.138433,
      "seconds": 34.835875444
    }
  },
  "by_version": {
    "v1": {
      "calls": 5,
      "input_tokens": 20,
      "output_tokens": 13320,
      "cache_read_tokens": 0,
      "cache_write_tokens": 18175,
      "cost_usd": 0.208525425,
      "seconds": 86.822649156
    },
    "v2": {
      "calls": 8,
      "input_tokens": 12412,
      "output_tokens": 12304,
      "cache_read_tokens": 4442,
      "cache_write_tokens": 24120,
      "cost_usd": 0.33322070000000004,
      "seconds": 100.143939002
    },
    "v3": {
      "calls": 34,
      "input_tokens": 47825,
      "output_tokens": 38790,
      "cache_read_tokens": 1615,
      "cache_write_tokens": 20680,
      "cost_usd": 0.5142661,
      "seconds": 264.71904244000007
    },
    "v3-all-opus": {
      "calls": 5,
      "input_tokens": 14205,
      "output_tokens": 11372,
      "cache_read_tokens": 0,
      "cache_write_tokens": 4672,
      "cost_usd": 0.30762,
      "seconds": 105.212745563
    },
    "v3.1": {
      "calls": 11,
      "input_tokens": 25199,
      "output_tokens": 11272,
      "cache_read_tokens": 3440,
      "cache_write_tokens": 43472,
      "cost_usd": 0.3348202,
      "seconds": 83.16957950700001
    },
    "v3.2": {
      "calls": 8,
      "input_tokens": 17927,
      "output_tokens": 7267,
      "cache_read_tokens": 1877,
      "cache_write_tokens": 33465,
      "cost_usd": 0.2618612,
      "seconds": 61.906540320999994
    },
    "v3.3": {
      "calls": 6,
      "input_tokens": 19645,
      "output_tokens": 8608,
      "cache_read_tokens": 0,
      "cache_write_tokens": 5541,
      "cost_usd": 0.173197,
      "seconds": 62.90693932800001
    }
  },
  "cache_read_share": 0.03579401536942429,
  "generated_at": "2026-11-11T19:36:27Z",
  "harness_total": {
    "calls": 20,
    "input_tokens": 52651,
    "output_tokens": 23655,
    "cache_read_tokens": 3335,
    "cache_write_tokens": 37186,
    "cost_usd": 0.5093359000000001,
    "seconds": 177.334188467
  },
  "issues": [
    {
      "issue": 1,
      "title": "Set a device's status (up/down) without re-creating it",
      "harness": {
        "calls": 6,
        "input_tokens": 13750,
        "output_tokens": 8565,
        "cache_read_tokens": 1615,
        "cache_write_tokens": 15693,
        "cost_usd": 0.17675310000000002,
        "seconds": 63.49714465
      },
      "naive": {
        "calls": 1,
        "input_tokens": 4640,
        "output_tokens": 7032,
        "cache_read_tokens": 0,
        "cache_write_tokens": 0,
        "cost_usd": 0.1592,
        "seconds": 50.484309353
      },
      "harness_gates_passed": true,
      "harness_qa_approved": true,
      "naive_gates_passed": true,
      "all_opus": {
        "calls": 5,
        "input_tokens": 14205,
        "output_tokens": 11372,
        "cache_read_tokens": 0,
        "cache_write_tokens": 4672,
        "cost_usd": 0.30762,
        "seconds": 105.212745563
      },
      "all_opus_qa_approved": true,
      "developer_models": [
        "claude-sonnet-5-5",
        "claude-sonnet-5-5"
      ],
      "developer_attempts": 2,
      "qa_rounds": 1,
      "files_changed": 3
    },
    {
      "issue": 2,
      "title": "Filter the device list by site and role",
      "harness": {
        "calls": 8,
        "input_tokens": 19256,
        "output_tokens": 6482,
        "cache_read_tokens": 1720,
        "cache_write_tokens": 15952,
        "cost_usd": 0.15938580000000002,
        "seconds": 50.930104489
      },
      "naive": {
        "calls": 0,
        "input_tokens": 0,
        "output_tokens": 0,
        "cache_read_tokens": 0,
        "cache_write_tokens": 0,
        "cost_usd": 0,
        "seconds": 0
      },
      "harness_gates_passed": true,
      "harness_qa_approved": true,
      "naive_gates_passed": false,
      "all_opus": {
        "calls": 0,
        "input_tokens": 0,
        "output_tokens": 0,
        "cache_read_tokens": 0,
        "cache_write_tokens": 0,
        "cost_usd": 0,
        "seconds": 0
      },
      "all_opus_qa_approved": false,
      "developer_models": [
        "claude-sonnet-5-5",
        "claude-sonnet-5-5"
      ],
      "developer_attempts": 2,
      "qa_rounds": 1,
      "files_changed": 3
    },
    {
      "issue": 3,
      "title": "Validate management IP addresses",
      "harness": {
        "calls": 6,
        "input_tokens": 19645,
        "output_tokens": 8608,
        "cache_read_tokens": 0,
        "cache_write_tokens": 5541,
        "cost_usd": 0.173197,
        "seconds": 62.90693932800001
      },
      "naive": {
        "calls": 0,
        "input_tokens": 0,
        "output_tokens": 0,
        "cache_read_tokens": 0,
        "cache_write_tokens": 0,
        "cost_usd": 0,
        "seconds": 0
      },
      "harness_gates_passed": true,
      "harness_qa_approved": true,
      "naive_gates_passed": false,
      "all_opus": {
        "calls": 0,
        "input_tokens": 0,
        "output_tokens": 0,
        "cache_read_tokens": 0,
        "cache_write_tokens": 0,
        "cost_usd": 0,
        "seconds": 0
      },
      "all_opus_qa_approved": false,
      "developer_models": [
        "claude-sonnet-5-5"
      ],
      "developer_attempts": 1,
      "qa_rounds": 1,
      "files_changed": 2
    }
  ],
  "merged_prs": 3,
  "naive_total": {
    "calls": 1,
    "input_tokens": 4640,
    "output_tokens": 7032,
    "cache_read_tokens": 0,
    "cache_write_tokens": 0,
    "cost_usd": 0.1592,
    "seconds": 50.484309353
  },
  "pr_open_to_merge_h": [
    0.13194444444444445,
    0.3227777777777778,
    0.11222222222222222
  ],
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
  "prs": [
    {
      "number": 6,
      "title": "fix(inventory): validate mgmt_ip as IPv4/IPv6 on create and update",
      "state": "MERGED",
      "createdAt": "2026-11-11T19:27:52Z",
      "mergedAt": "2026-11-11T19:35:47Z",
      "labels": [
        {
          "name": "ai-authored"
        }
      ],
      "additions": 115,
      "deletions": 0,
      "statusCheckRollup": [
        {
          "name": "test",
          "conclusion": "SUCCESS"
        },
        {
          "name": "lint",
          "conclusion": "SUCCESS"
        },
        {
          "name": "security",
          "conclusion": "SUCCESS"
        },
        {
          "name": "image",
          "conclusion": "SUCCESS"
        },
        {
          "name": "release-readiness",
          "conclusion": "SUCCESS"
        }
      ]
    },
    {
      "number": 5,
      "title": "feat(inventory): filter GET /devices by site and role",
      "state": "MERGED",
      "createdAt": "2026-11-11T19:02:34Z",
      "mergedAt": "2026-11-11T19:21:56Z",
      "labels": [
        {
          "name": "ai-authored"
        }
      ],
      "additions": 98,
      "deletions": 2,
      "statusCheckRollup": [
        {
          "name": "test",
          "conclusion": "SUCCESS"
        },
        {
          "name": "lint",
          "conclusion": "SUCCESS"
        },
        {
          "name": "security",
          "conclusion": "SUCCESS"
        },
        {
          "name": "image",
          "conclusion": "SUCCESS"
        },
        {
          "name": "release-readiness",
          "conclusion": "SUCCESS"
        }
      ]
    },
    {
      "number": 4,
      "title": "feat(inventory): add endpoint to set device status up/down",
      "state": "MERGED",
      "createdAt": "2026-11-11T18:53:53Z",
      "mergedAt": "2026-11-11T19:00:37Z",
      "labels": [
        {
          "name": "ai-authored"
        }
      ],
      "additions": 146,
      "deletions": 0,
      "statusCheckRollup": [
        {
          "name": "test",
          "conclusion": "SUCCESS"
        },
        {
          "name": "lint",
          "conclusion": "SUCCESS"
        },
        {
          "name": "security",
          "conclusion": "SUCCESS"
        },
        {
          "name": "image",
          "conclusion": "SUCCESS"
        },
        {
          "name": "release-readiness",
          "conclusion": "SUCCESS"
        }
      ]
    }
  ],
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
