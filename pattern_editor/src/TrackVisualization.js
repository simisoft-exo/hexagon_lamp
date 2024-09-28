import React, { useRef, useCallback } from 'react';

function TrackVisualization({ track, updateSegment, selectedSegment, setSelectedSegment }) {
  const totalDuration = track.reduce((sum, segment) => sum + segment.duration, 0);
  const maxSpeed = 6;
  const containerRef = useRef(null);

  const handleSpeedDrag = useCallback((event, segmentId, initialY) => {
    if (!containerRef.current) return;
    const containerHeight = containerRef.current.clientHeight;
    const deltaY = initialY - event.clientY;
    const speedChange = (deltaY / containerHeight) * 100;
    
    const segment = track.find(seg => seg.id === segmentId);
    if (!segment) return;

    const newSpeed = Math.max(0, Math.min(100, segment.speed + speedChange));
    updateSegment(segmentId, 'speed', Math.round(newSpeed));
  }, [track, updateSegment]);

  const handleDurationDrag = useCallback((event, segmentId, initialX) => {
    if (!containerRef.current) return;
    const containerWidth = containerRef.current.clientWidth;
    const deltaX = event.clientX - initialX;
    const durationChange = (deltaX / containerWidth) * totalDuration;
    
    const segment = track.find(seg => seg.id === segmentId);
    if (!segment) return;

    const newDuration = Math.max(100, Math.min(30000, segment.duration + durationChange));
    updateSegment(segmentId, 'duration', Math.round(newDuration));
  }, [track, totalDuration, updateSegment]);

  return (
    <div className="track-visualization my-4 p-4 border-2 border-gray-300 rounded-lg">
      <h2 className="text-xl font-semibold mb-2">Track Visualization</h2>
      <div ref={containerRef} className="relative h-40 bg-gray-200 rounded">
        {track.map((segment, index) => {
          const width = (segment.duration / totalDuration) * 100;
          const height = ((segment.speed * 0.06) / maxSpeed) * 100;
          const left = track
            .slice(0, index)
            .reduce((sum, seg) => sum + (seg.duration / totalDuration) * 100, 0);
          return (
            <div
              key={segment.id}
              className={`absolute bottom-0 rounded-t-lg ${selectedSegment === segment.id ? 'bg-blue-700' : 'bg-blue-500'}`}
              style={{
                left: `${left}%`,
                width: `${width}%`,
                height: `${height}%`,
              }}
              onClick={() => setSelectedSegment(segment.id)}
            >
              <div className="text-xs text-white p-1">
                {segment.duration}ms, {(segment.speed * 0.06).toFixed(2)}
              </div>
              {selectedSegment === segment.id && (
                <>
                  <div 
                    className="absolute top-0 left-0 w-full h-2 bg-yellow-300 cursor-ns-resize"
                    onMouseDown={(e) => {
                      const initialY = e.clientY;
                      const handleMouseMove = (moveEvent) => {
                        handleSpeedDrag(moveEvent, segment.id, initialY);
                      };
                      const handleMouseUp = () => {
                        document.removeEventListener('mousemove', handleMouseMove);
                        document.removeEventListener('mouseup', handleMouseUp);
                      };
                      document.addEventListener('mousemove', handleMouseMove);
                      document.addEventListener('mouseup', handleMouseUp);
                    }}
                  />
                  <div 
                    className="absolute top-0 right-0 w-2 h-full bg-green-300 cursor-ew-resize"
                    onMouseDown={(e) => {
                      const initialX = e.clientX;
                      const handleMouseMove = (moveEvent) => {
                        handleDurationDrag(moveEvent, segment.id, initialX);
                      };
                      const handleMouseUp = () => {
                        document.removeEventListener('mousemove', handleMouseMove);
                        document.removeEventListener('mouseup', handleMouseUp);
                      };
                      document.addEventListener('mousemove', handleMouseMove);
                      document.addEventListener('mouseup', handleMouseUp);
                    }}
                  />
                </>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}

export default TrackVisualization;