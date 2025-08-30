# TaskWing AI Interaction Examples

Practical examples of using TaskWing with AI assistants (Claude Code, Cursor, etc.) via MCP.

## Getting Started Examples

### First-Time Setup

**You**: I just installed TaskWing. Can you help me set it up and create my first task?

**AI**: I'll help you get started with TaskWing! Let me check if you have any existing tasks and help you create your first one.

_[AI uses task-summary, then guides through initialization and task creation]_

**Result**:

- Project initialized with `.taskwing/` directory
- First task created with proper title and priority
- Current task context set for future interactions

### Project Initialization

**You**: I'm starting a new React app project. Can you help me plan the initial tasks?

**AI**: I'll create a comprehensive task breakdown for your React app project.

_[AI uses batch-create-tasks to create related tasks with dependencies]_

**Tasks Created**:

```
✅ Project setup and configuration (high priority)
├── Initialize React app with TypeScript
├── Set up ESLint and Prettier
├── Configure CI/CD pipeline
└── Set up testing framework

🎨 UI Development (depends on setup)
├── Create component library
├── Implement responsive layout
├── Add routing with React Router
└── Style with Tailwind CSS

🔐 Authentication (high priority)
├── Set up auth context
├── Implement login/logout
├── Add protected routes
└── Handle token management
```

## Daily Development Workflow

### Morning Check-in

**You**: What should I work on today?

**AI**: Let me check your current project status and suggest priorities.

_[AI uses get-current-task, task-summary, suggest-tasks]_

**Response**:

```
📊 Project Status:
• Current Task: "Implement user authentication" (in progress)
• Completed This Week: 7/12 tasks
• High Priority Ready: 3 tasks

🎯 Today's Recommendations:
1. **Continue**: "Implement user authentication" (80% complete)
2. **Next**: "Set up protected routes" (depends on auth, high priority)
3. **Quick Win**: "Add loading states" (no dependencies, 30min)

Would you like me to update your current task or create any additional tasks?
```

### Task Progress Update

**You**: I finished the authentication implementation. What's next?

**AI**: Great progress! Let me mark that complete and suggest what to work on next.

_[AI uses mark-done, set-current-task, suggest-tasks]_

**Actions Taken**:

- ✅ Marked "Implement user authentication" as done
- 🎯 Set current task to "Set up protected routes"
- 📋 Updated 3 dependent tasks from "blocked" to "ready"
- 🎉 Project is now 75% complete for this sprint

### End of Day Review

**You**: Can you show me what I accomplished today?

**AI**: Here's your daily progress summary:

_[AI uses task-analytics with date filtering]_

**Today's Accomplishments**:

```
✅ Completed Tasks (3):
• Implement OAuth2 authentication
• Add login form validation
• Set up JWT token handling

🔄 In Progress (1):
• Set up protected routes (75% complete)

⏱️ Time Breakdown:
• Authentication: 4.5 hours
• UI components: 1.5 hours
• Bug fixes: 0.5 hours

📈 Sprint Progress: 9/12 tasks complete (75%)
```

## Project Planning Examples

### Feature Breakdown

**You**: I need to add a commenting system to my blog app. Can you break this down into tasks?

**AI**: I'll break down the commenting system into manageable development tasks.

_[AI uses batch-create-tasks with proper dependencies]_

**Task Breakdown**:

```
💾 Backend (High Priority):
├── Design comment database schema
├── Create comment API endpoints
├── Add comment validation
├── Implement comment moderation
└── Add email notifications

🎨 Frontend (Depends on Backend):
├── Create comment component
├── Add comment form with validation
├── Implement real-time updates
├── Add moderation interface
└── Style comment threads

🧪 Testing & Polish:
├── Write API tests
├── Add component tests
├── Performance testing
└── Accessibility audit
```

### Sprint Planning

**You**: We have a 2-week sprint starting Monday. Can you help me plan based on our current tasks?

**AI**: I'll analyze your current tasks and create an optimal 2-week sprint plan.

_[AI uses workflow-status, task-analytics, board-snapshot]_

**Sprint Plan**:

```
📅 Week 1 Focus (High-value features):
• Complete user management system (3 tasks remaining)
• Implement payment processing (new feature)
• Fix 2 critical bugs in production

📅 Week 2 Focus (Polish & prep):
• Add comprehensive testing
• Optimize performance
• Prepare deployment pipeline
• Documentation updates

🎯 Sprint Goals:
• ✅ 15 planned tasks
• 🎯 2 major features shipped
• 🐛 0 critical bugs remaining
• 📊 Target: 85% task completion rate

⚠️ Risks Identified:
• Payment API integration may need extra time
• Dependency on external design review
```

## Problem-Solving Examples

### Bug Tracking

**You**: We found a critical bug in production. Help me track the investigation and fix.

**AI**: I'll create tasks to systematically investigate and fix this production issue.

_[AI uses add-task with urgent priority and proper structure]_

**Bug Investigation Plan**:

```
🚨 Critical Bug: User login fails intermittently

🔍 Investigation Tasks:
├── Analyze server logs for error patterns (urgent)
├── Reproduce bug in staging environment (urgent)
├── Check database connection stability
└── Review recent authentication changes

🛠️ Fix Tasks (created after investigation):
├── Implement proper error handling
├── Add retry logic for API calls
├── Update monitoring and alerting
└── Deploy hotfix to production

📋 Follow-up:
├── Post-mortem analysis
├── Improve testing coverage
└── Update deployment checklist
```

### Technical Debt

**You**: Our codebase has accumulated technical debt. Help me prioritize what to tackle first.

**AI**: I'll help you create a systematic approach to addressing technical debt.

_[AI analyzes project context and creates prioritized technical debt tasks]_

**Technical Debt Roadmap**:

```
🔥 High Impact (Do First):
├── Refactor authentication module (security risk)
├── Fix memory leaks in dashboard (performance)
├── Update deprecated dependencies (compatibility)
└── Add error boundaries (user experience)

⚡ Medium Impact (Next Sprint):
├── Consolidate duplicate API calls
├── Improve component reusability
├── Add proper TypeScript types
└── Optimize bundle size

🧹 Low Impact (Background work):
├── Update code comments
├── Standardize naming conventions
├── Remove unused code
└── Improve file organization

📊 Effort vs Impact Analysis:
• High-impact items: 32 hours estimated
• Quick wins available: 6 tasks under 2 hours each
• Long-term payoff: Reduced maintenance by 40%
```

## Advanced Usage Examples

### Cross-Project Dependencies

**You**: I'm working on multiple projects that depend on each other. How can TaskWing help?

**AI**: I'll help you manage cross-project dependencies using TaskWing's linking features.

_[AI creates tasks across projects with external dependencies]_

**Project Structure**:

```
📁 Shared Component Library:
├── Build design system components (blocks: API project)
├── Publish to npm registry
├── Create documentation site
└── Set up automated testing

📁 API Backend:
├── Design API schema (ready)
├── Implement core endpoints (depends on: auth service)
├── Add rate limiting
└── Deploy to staging (blocks: frontend)

📁 Frontend App:
├── Integrate component library (depends on: library v1.0)
├── Connect to API endpoints (depends on: API staging)
├── Add user authentication
└── Production deployment (depends on: all above)

🔗 Dependency Chain:
Auth Service → API Backend → Frontend → User Testing
             ↘ Component Library ↗
```

### Team Coordination

**You**: Our team of 4 developers needs to coordinate on a large feature. Help us organize the work.

**AI**: I'll create a coordinated task breakdown that accounts for team dependencies and parallel work.

_[AI uses batch-create-tasks with team assignments and dependencies]_

**Team Coordination Plan**:

```
👨‍💻 Alice (Backend Lead):
├── Design database migrations
├── Implement core API endpoints
├── Set up authentication middleware
└── Create API documentation

👩‍💻 Bob (Frontend Lead):
├── Create UI component library
├── Implement state management
├── Build user interface (depends on: API)
└── Add responsive design

🧪 Charlie (QA Engineer):
├── Write API integration tests
├── Create frontend test suite
├── Set up automated testing
└── Performance testing (depends on: full feature)

🚀 Dana (DevOps):
├── Set up staging environment
├── Configure CI/CD pipeline
├── Implement monitoring
└── Plan production deployment

📅 Timeline Coordination:
Week 1: Alice & Dana (infrastructure)
Week 2: Alice & Bob (parallel development)
Week 3: Charlie (testing), Dana (deployment prep)
Week 4: Full team (integration & launch)
```

## Troubleshooting Examples

### Context Recovery

**You**: I lost track of what I was working on. Can you help me figure out where I left off?

**AI**: Let me analyze your recent activity and current project state to help you get back on track.

_[AI uses get-current-task, task-summary, recent activity analysis]_

**Context Recovery**:

```
🎯 Your Last Session:
• Working on: "Implement password reset flow"
• Status: 60% complete (in review)
• Last update: 2 hours ago
• Blocked by: Email service integration

📋 What You Completed Recently:
✅ Set up email templates (yesterday)
✅ Added form validation (yesterday)
✅ Created password reset API (2 days ago)

🚀 Recommended Next Steps:
1. **Continue**: Finish email service integration (30min)
2. **Test**: Validate password reset flow end-to-end
3. **Deploy**: Push to staging for team review

🔍 Context Clues Found:
• Branch: feature/password-reset (3 commits ahead)
• Draft email in Gmail about email service setup
• Slack message asking about SMTP configuration
```

### Project Recovery

**You**: I haven't worked on this project in 3 weeks. Help me get back up to speed.

**AI**: I'll help you understand the current state and what needs attention after your break.

_[AI provides comprehensive project status and reentry plan]_

**Project Reentry Brief**:

```
📊 Project Status (3-week summary):
• Total Tasks: 45 (was 38 when you left)
• Completed: 28 (+8 since you left)
• In Progress: 6 (3 were blocked, now unblocked)
• New: 7 tasks added by team

🏆 Major Accomplishments (while away):
✅ Authentication system completed
✅ Payment integration shipped
✅ Mobile responsive design finished
✅ Performance optimizations deployed

🎯 Your Previous Work:
• Last worked on: "User dashboard analytics"
• Status: 75% complete, ready to resume
• Dependencies resolved: API endpoints now available
• Tests written: Ready for implementation

⚠️ Attention Needed:
• 2 bugs assigned to you (low priority)
• 1 code review pending your input
• Database migration needs your approval

📋 Recommended Reentry Plan:
Day 1: Review code changes, understand new architecture
Day 2: Resume dashboard analytics work
Day 3: Address pending reviews and minor bugs
Day 4+: Continue with planned feature development

💡 What Changed:
• New team member joined (handling mobile development)
• Architecture simplified (removed microservice complexity)
• Design system updated (new components available)
```

These examples show how TaskWing with AI integration becomes a powerful project management and development workflow tool, adapting to different scenarios and providing intelligent assistance throughout the development lifecycle.
