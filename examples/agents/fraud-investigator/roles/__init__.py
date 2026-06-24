"""Per-role implementations for the fraud-investigator demo.

Each role module exposes a `run(ctx)` callable. The shared `RoleContext`
in `common.py` carries the wired-up OpenAI client, OTel tracer, and SDK
config — keeping each role focused on its own framework code.
"""
