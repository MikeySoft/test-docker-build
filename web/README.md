# Flotilla Frontend

A modern React frontend for the Flotilla Docker management platform.

## Features

- **Host Management**: View and manage Docker hosts
- **Container Operations**: Start, stop, restart, and remove containers
- **Stack Management**: Deploy and manage Docker Compose stacks
- **Compose Editor**: Monaco editor with YAML syntax highlighting and validation
- **Real-time Updates**: Live status updates and notifications
- **Responsive Design**: Mobile-friendly interface

## Tech Stack

- **React 18** with TypeScript
- **Vite** for fast development and building
- **Tailwind CSS** for styling
- **React Router** for navigation
- **TanStack Query** for data fetching
- **Zustand** for state management
- **Monaco Editor** for code editing
- **Lucide React** for icons

## Development

### Prerequisites

- Node.js 18+
- npm or yarn

### Setup

```bash
# Install dependencies
npm install

# Start development server
npm run dev

# Build for production
npm run build

# Preview production build
npm run preview
```

### Development Server

The development server runs on `http://localhost:5173` and proxies API requests to `http://localhost:8080`.

## Project Structure

```
src/
├── api/           # API client and types
├── components/    # Reusable UI components
├── pages/         # Page components
├── stores/        # Zustand state stores
├── hooks/         # Custom React hooks
└── types/         # TypeScript type definitions
```

## Key Components

- **Layout**: Main application layout with navigation
- **HostList**: Grid view of all Docker hosts
- **HostDetail**: Detailed host view with tabs for containers, stacks, etc.
- **ComposeEditor**: Monaco editor for Docker Compose files
- **StatusBadge**: Reusable status indicator component
- **Modal**: Reusable modal component

## API Integration

The frontend communicates with the Flotilla backend via REST API and WebSocket connections:

- REST API endpoints for CRUD operations
- WebSocket for real-time updates
- Automatic retry and error handling
- Optimistic updates for better UX

## Styling

Uses Tailwind CSS with a custom design system:

- Primary colors for actions and highlights
- Success/warning/danger colors for status indicators
- Responsive grid layouts
- Consistent spacing and typography
- Dark mode support (future)

## State Management

Uses Zustand for lightweight state management:

- Host and container state
- WebSocket connection status
- UI state (loading, errors)
- Real-time updates

## Future Enhancements

- Container logs viewer with xterm.js
- Real-time metrics and monitoring
- User authentication and RBAC
- Dark mode theme
- Advanced filtering and search
- Bulk operations
- Container terminal access