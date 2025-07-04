import { memo } from 'react'
import { Handle, Position, NodeProps } from '@reactflow/core'

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
    if (use.includes('echo')) return 'bg-blue-100 border-blue-300 text-blue-800'
    if (use.includes('http')) return 'bg-red-100 border-red-300 text-red-800'
    if (use.includes('openai')) return 'bg-green-100 border-green-300 text-green-800'
    if (use.includes('anthropic')) return 'bg-orange-100 border-orange-300 text-orange-800'
    if (use.includes('slack')) return 'bg-purple-100 border-purple-300 text-purple-800'
    if (use.includes('twilio')) return 'bg-pink-100 border-pink-300 text-pink-800'
    return 'bg-gray-100 border-gray-300 text-gray-800'
  }

  const selectedClass = selected ? 'ring-2 ring-blue-500 ring-offset-2' : ''

  return (
    <div 
      className={`px-4 py-3 shadow-lg rounded-lg border-2 min-w-[180px] ${getNodeColor(data.use)} ${selectedClass}`}
      style={{ backgroundColor: 'white' }}
    >
      <Handle
        type="target"
        position={Position.Top}
        className="w-3 h-3 bg-gray-400 border-2 border-white"
      />

      <div className="flex items-center gap-3 mb-2">
        <span className="text-xl">{getNodeIcon(data.use)}</span>
        <div className="flex-1">
          <div className="font-semibold text-sm leading-tight">{data.id}</div>
          <div className="text-xs opacity-75 leading-tight">{data.use}</div>
        </div>
      </div>

      {data.with && Object.keys(data.with).length > 0 && (
        <div className="text-xs opacity-75 mb-1">
          {Object.keys(data.with).length} parameter{Object.keys(data.with).length !== 1 ? 's' : ''}
        </div>
      )}

      {data.if && (
        <div className="text-xs bg-yellow-200 text-yellow-800 px-2 py-1 rounded mt-1">
          Conditional
        </div>
      )}

      <Handle
        type="source"
        position={Position.Bottom}
        className="w-3 h-3 bg-gray-400 border-2 border-white"
      />
    </div>
  )
})

StepNode.displayName = 'StepNode'