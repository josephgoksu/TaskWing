# TaskWing Landing Page

Marketing website for TaskWing, the AI-powered CLI task manager.

Built with React + TypeScript + Vite for fast development and optimal performance.

## Development

```bash
# Install dependencies
bun install

# Start development server
bun run dev

# Build for production
bun run build

# Preview production build
bun run preview
```

## Features

- **Responsive Design**: Works on all device sizes
- **Dark Mode**: Automatic dark/light mode with user preference
- **Interactive Components**: Installation wizard, GitHub stats, FAQ section
- **Performance Optimized**: Fast loading with code splitting
- **Analytics Ready**: Built-in analytics hooks for tracking

## Tech Stack

- **React 18** with TypeScript
- **Vite** for build tooling and dev server
- **CSS Modules** for styling
- **Responsive Design** with CSS Grid and Flexbox

## Project Structure

```
src/
├── components/          # Reusable UI components
├── hooks/              # Custom React hooks
├── utils/              # Utility functions
└── assets/             # Static assets
```

## Building

The site builds to static files that can be deployed anywhere:

```bash
bun run build
```

Output goes to `dist/` directory.

## Related

- [Main Project](../README.md) - TaskWing CLI tool
- [Documentation](../DOCS.md) - Complete user guide
- [MCP Integration](../MCP.md) - AI tool setup
