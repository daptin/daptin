# SESSION 5 PROMPT: Workflow & Automation Features

Continue the Daptin self-documentation project. This is SESSION 5 of 7 - focus on Workflow & Automation features.

## CONTEXT:
- Sessions 1-4 completed
- Current progress: 71% (37/52 features documented)
- Server running on port 6336 with admin@test.com/testpass123
- WebSocket auth solution: use ?token=TOKEN in URL
- Configuration via /_config/backend/{key}
- GraphQL requires config enable + restart

## CRITICAL LEARNINGS FROM PREVIOUS SESSIONS:
- WebSocket requires query param auth, not headers
- Configuration changes via /_config API
- Some changes need restart (world schema, actions, GraphQL)
- Import/Export via actions, not REST endpoints
- Rate limiting and caching are built-in
- CORS fully configurable
- Multi-site architecture supports subsites

## GOAL: Document 8 workflow features to reach 86% completion (45/52 features).

## EXECUTION PLAN (from SELF_DOCUMENTATION_MASTER_PLAN.md):
1. **Actions System Deep Dive** - Test all action types and patterns
2. **State Machines** - FSM configuration and usage
3. **Task Scheduler** - Background job management
4. **Integration System** - External service connections
5. **OAuth Providers** - Social login implementation

## SPECIFIC TASKS:
1. Read todo list and review progress
2. Get fresh JWT token
3. Test action system:
   - List all available actions
   - Test default actions (signup, signin, etc.)
   - Create custom actions
   - Action permissions and outcomes
4. Test state machines:
   - State machine configuration
   - State transitions
   - Event triggers
   - Audit logging
5. Test task scheduler:
   - Create scheduled tasks
   - Cron expressions
   - Task execution logs
6. Test integrations:
   - OAuth provider setup
   - External API connections
   - Webhook configuration
7. Test exchange contracts:
   - Data sync patterns
   - Import/export automation
8. Document workflow patterns

## TARGET: Reach 86% completion (45/52 features) with working examples.

## NOTES:
- Focus on automation capabilities
- Document real-world workflow examples
- Test everything operationally
- Update OpenAPI with new findings

Remember: Every example must work. Test everything operationally.