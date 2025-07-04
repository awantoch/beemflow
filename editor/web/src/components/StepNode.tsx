import { memo } from 'react'
import { Handle, Position, NodeProps } from 'reactflow'

interface StepNodeData {
  id: string
  use: string
  with?: Record<string, any>
  if?: string
}

export const StepNode = memo(({ data, selected }: NodeProps<StepNodeData>) => {
  const getNodeIcon = (use: string) => {
    if (use.includes('echo')) return '📢'
    if (use.includes('http')) return '🌐'
    if (use.includes('openai')) return '🤖'
    if (use.includes('anthropic')) return '🧠'
    if (use.includes('slack')) return '💬'
    if (use.includes('twilio')) return '📱'
    return '⚙️'
  }

  const getNodeColor = (use: string) => {
    if (use.includes('echo')) return '#dbeafe #93c5fd #1e40af' // blue
    if (use.includes('http')) return '#fee2e2 #fca5a5 #991b1b' // red
    if (use.includes('openai')) return '#dcfce7 #86efac #166534' // green
    if (use.includes('anthropic')) return '#fed7aa #fdba74 #9a3412' // orange
    if (use.includes('slack')) return '#f3e8ff #c4b5fd #6b21a8' // purple
    if (use.includes('twilio')) return '#fce7f3 #f9a8d4 #9d174d' // pink
    return '#f3f4f6 #d1d5db #374151' // gray
  }

  const [bgColor, borderColor, textColor] = getNodeColor(data.use).split(' ')

  return (
    <div 
      style={{
        backgroundColor: bgColor,
        borderColor: borderColor,
        color: textColor,
        boxShadow: selected ? '0 0 0 2px #3b82f6, 0 0 0 4px rgba(59, 130, 246, 0.1)' : '0 10px 15px -3px rgba(0, 0, 0, 0.1)',
        borderWidth: '2px',
        borderStyle: 'solid',
        borderRadius: '8px',
        padding: '12px 16px',
        minWidth: '180px',
        fontSize: '14px',
        fontFamily: 'system-ui, sans-serif'
      }}
    >
      <Handle
        type="target"
        position={Position.Top}
        style={{
          width: '12px',
          height: '12px',
          backgroundColor: '#6b7280',
          border: '2px solid white',
          borderRadius: '50%',
          top: '-6px'
        }}
      />

      <div style={{ display: 'flex', alignItems: 'center', gap: '12px', marginBottom: '8px' }}>
        <span style={{ fontSize: '18px' }}>{getNodeIcon(data.use)}</span>
        <div style={{ flex: 1 }}>
          <div style={{ fontWeight: 600, lineHeight: '1.25' }}>{data.id}</div>
          <div style={{ opacity: 0.75, fontSize: '12px', lineHeight: '1.25' }}>{data.use}</div>
        </div>
      </div>

      {data.with && Object.keys(data.with).length > 0 && (
        <div style={{ opacity: 0.75, fontSize: '12px', marginBottom: '4px' }}>
          {Object.keys(data.with).length} parameter{Object.keys(data.with).length !== 1 ? 's' : ''}
        </div>
      )}

      {data.if && (
        <div style={{
          fontSize: '12px',
          backgroundColor: '#fef08a',
          color: '#854d0e',
          padding: '2px 8px',
          borderRadius: '4px',
          marginTop: '4px'
        }}>
          Conditional
        </div>
      )}

      <Handle
        type="source"
        position={Position.Bottom}
        style={{
          width: '12px',
          height: '12px',
          backgroundColor: '#6b7280',
          border: '2px solid white',
          borderRadius: '50%',
          bottom: '-6px'
        }}
      />
    </div>
  )
})

StepNode.displayName = 'StepNode'