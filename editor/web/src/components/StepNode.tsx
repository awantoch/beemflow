import { memo, useMemo } from 'react'
import { Handle, Position, NodeProps } from 'reactflow'

interface StepNodeData {
  id: string
  use: string
  with?: Record<string, any>
  if?: string
}

// Node configuration
const NODE_CONFIG = {
  HANDLE_STYLE: { width: 8, height: 8 },
  DEFAULT_WIDTH: 200,
  DEFAULT_HEIGHT: 80,
} as const

// Step type definitions with consistent styling
const STEP_TYPES = {
  echo: { icon: '📢', colors: ['#dbeafe', '#93c5fd', '#1e40af'] },
  http: { icon: '🌐', colors: ['#fee2e2', '#fca5a5', '#991b1b'] },
  openai: { icon: '🤖', colors: ['#dcfce7', '#86efac', '#166534'] },
  anthropic: { icon: '🧠', colors: ['#fef3c7', '#fbbf24', '#92400e'] },
  slack: { icon: '💬', colors: ['#f3e8ff', '#c084fc', '#7c3aed'] },
  twilio: { icon: '📱', colors: ['#fce7f3', '#f9a8d4', '#be185d'] },
  default: { icon: '⚙️', colors: ['#f3f4f6', '#9ca3af', '#374151'] },
} as const

type StepType = keyof typeof STEP_TYPES

export const StepNode = memo(({ data, selected }: NodeProps<StepNodeData>) => {
  const stepType = useMemo((): StepType => {
    const use = data.use.toLowerCase()
    
    for (const [key] of Object.entries(STEP_TYPES)) {
      if (key !== 'default' && use.includes(key)) {
        return key as StepType
      }
    }
    return 'default'
  }, [data.use])

  const { icon, colors } = STEP_TYPES[stepType]
  const [bgColor, borderColor, textColor] = colors

  const hasCondition = Boolean(data.if)
  const hasParameters = Boolean(data.with && Object.keys(data.with).length > 0)

  const nodeStyle = useMemo(() => ({
    background: bgColor,
    border: `2px solid ${selected ? textColor : borderColor}`,
    borderRadius: '8px',
    padding: '12px',
    minWidth: NODE_CONFIG.DEFAULT_WIDTH,
    minHeight: NODE_CONFIG.DEFAULT_HEIGHT,
    color: textColor,
    fontSize: '14px',
    fontFamily: 'system-ui, sans-serif',
    boxShadow: selected 
      ? `0 0 0 2px ${textColor}33` 
      : '0 1px 3px rgba(0, 0, 0, 0.1)',
    transition: 'all 0.2s ease',
  }), [bgColor, borderColor, textColor, selected])

  const headerStyle = useMemo(() => ({
    display: 'flex',
    alignItems: 'center',
    gap: '8px',
    marginBottom: '8px',
    fontWeight: 600,
  }), [])

  const badgeStyle = useMemo(() => ({
    fontSize: '10px',
    padding: '2px 6px',
    borderRadius: '4px',
    backgroundColor: textColor,
    color: bgColor,
    fontWeight: 500,
  }), [textColor, bgColor])

  const renderParameterSummary = useMemo(() => {
    if (!hasParameters) return null

    const paramCount = Object.keys(data.with!).length
    const firstParam = Object.entries(data.with!)[0]
    
    return (
      <div style={{ 
        fontSize: '12px', 
        opacity: 0.8, 
        marginTop: '4px',
        wordBreak: 'break-word'
      }}>
        {paramCount === 1 ? (
          <span>{firstParam[0]}: {String(firstParam[1]).slice(0, 20)}...</span>
        ) : (
          <span>{paramCount} parameters</span>
        )}
      </div>
    )
  }, [hasParameters, data.with])

  return (
    <div style={nodeStyle}>
      <Handle
        type="target"
        position={Position.Top}
        style={NODE_CONFIG.HANDLE_STYLE}
      />
      
      <div style={headerStyle}>
        <span style={{ fontSize: '16px' }}>{icon}</span>
        <span>{data.id}</span>
        {hasCondition && (
          <span style={badgeStyle}>IF</span>
        )}
      </div>
      
      <div style={{ 
        fontSize: '12px', 
        opacity: 0.7, 
        marginBottom: '4px',
        wordBreak: 'break-word'
      }}>
        {data.use}
      </div>
      
      {renderParameterSummary}
      
      <Handle
        type="source"
        position={Position.Bottom}
        style={NODE_CONFIG.HANDLE_STYLE}
      />
    </div>
  )
})

StepNode.displayName = 'StepNode'