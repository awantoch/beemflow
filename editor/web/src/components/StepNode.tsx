import { memo } from 'react'
import { Handle, Position, NodeProps } from 'reactflow'

interface StepNodeData {
  id: string
  use?: string
  with?: Record<string, any>
  if?: string
}

// Simple step type detection
const getStepInfo = (use: string = '') => {
  const lowerUse = use.toLowerCase()
  
  if (lowerUse.includes('echo')) return { icon: '📢', color: '#3b82f6' }
  if (lowerUse.includes('http')) return { icon: '🌐', color: '#ef4444' }
  if (lowerUse.includes('openai')) return { icon: '🤖', color: '#10b981' }
  if (lowerUse.includes('anthropic')) return { icon: '🧠', color: '#f59e0b' }
  if (lowerUse.includes('slack')) return { icon: '💬', color: '#8b5cf6' }
  if (lowerUse.includes('twilio')) return { icon: '📱', color: '#ec4899' }
  
  return { icon: '⚙️', color: '#6b7280' }
}

export const StepNode = memo(({ data, selected }: NodeProps<StepNodeData>) => {
  const { icon, color } = getStepInfo(data.use)
  const hasCondition = Boolean(data.if)
  const hasParameters = Boolean(data.with && Object.keys(data.with).length > 0)

  return (
    <div 
      style={{
        background: 'white',
        border: `2px solid ${selected ? color : '#e5e7eb'}`,
        borderRadius: '8px',
        padding: '12px',
        minWidth: '180px',
        fontSize: '14px',
        boxShadow: selected 
          ? `0 0 0 2px ${color}33` 
          : '0 2px 4px rgba(0,0,0,0.1)',
      }}
    >
      <Handle
        type="target"
        position={Position.Top}
        style={{ width: 8, height: 8 }}
      />
      
      {/* Header */}
      <div style={{ 
        display: 'flex', 
        alignItems: 'center', 
        gap: '8px', 
        marginBottom: '8px',
        fontWeight: 600,
      }}>
        <span style={{ fontSize: '16px' }}>{icon}</span>
        <span>{data.id}</span>
        {hasCondition && (
          <span style={{
            fontSize: '10px',
            padding: '2px 6px',
            borderRadius: '4px',
            backgroundColor: color,
            color: 'white',
            fontWeight: 500,
          }}>
            IF
          </span>
        )}
      </div>
      
      {/* Use */}
      {data.use && (
        <div style={{ 
          fontSize: '12px', 
          color: '#6b7280',
          marginBottom: '4px',
        }}>
          {data.use}
        </div>
      )}
      
      {/* Parameters */}
      {hasParameters && (
        <div style={{ 
          fontSize: '12px', 
          color: '#9ca3af',
          marginTop: '4px',
        }}>
          {Object.keys(data.with!).length} parameter{Object.keys(data.with!).length !== 1 ? 's' : ''}
        </div>
      )}
      
      <Handle
        type="source"
        position={Position.Bottom}
        style={{ width: 8, height: 8 }}
      />
    </div>
  )
})

StepNode.displayName = 'StepNode'