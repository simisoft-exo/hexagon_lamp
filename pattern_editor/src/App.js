import React, { useState, useRef, useEffect } from 'react';
import './App.css';
import axios from 'axios';

function TrackVisualization({ track, updateSegment }) {
  const totalDuration = track.reduce((sum, segment) => sum + segment.duration, 0);
  const maxSpeed = 6;
  const containerRef = useRef(null);

  const handleSpeedDrag = (event, segmentId, initialY) => {
    if (!containerRef.current) return;

    const containerHeight = containerRef.current.clientHeight;
    const deltaY = initialY - event.clientY;
    const speedChange = (deltaY / containerHeight) * 100;
    
    const segment = track.find(seg => seg.id === segmentId);
    if (!segment) return;

    const newSpeed = Math.max(0, Math.min(100, segment.speed + speedChange));
    updateSegment(segmentId, 'speed', Math.round(newSpeed));
  };

  const handleDurationDrag = (event, segmentId, initialX) => {
    if (!containerRef.current) return;

    const containerWidth = containerRef.current.clientWidth;
    const deltaX = event.clientX - initialX;
    const durationChange = (deltaX / containerWidth) * totalDuration;
    
    const segment = track.find(seg => seg.id === segmentId);
    if (!segment) return;

    const newDuration = Math.max(100, Math.min(30000, segment.duration + durationChange));
    updateSegment(segmentId, 'duration', Math.round(newDuration));
  };

  return (
    <div className="track-visualization my-4 p-4 border-2 border-gray-300 rounded-lg">
      <h2 className="text-xl font-semibold mb-2">Track Visualization</h2>
      <div ref={containerRef} className="relative h-32 bg-gray-200 rounded">
        {track.map((segment, index) => {
          const width = (segment.duration / totalDuration) * 100;
          const height = ((segment.speed * 0.06) / maxSpeed) * 100;
          const left = track
            .slice(0, index)
            .reduce((sum, seg) => sum + (seg.duration / totalDuration) * 100, 0);
          return (
            <div
              key={segment.id}
              className="absolute bottom-0 rounded-t-lg bg-blue-500 border border-blue-600 cursor-move"
              style={{
                left: `${left}%`,
                width: `${width}%`,
                height: `${height}%`,
              }}
              onMouseDown={(e) => {
                const initialY = e.clientY;
                const initialX = e.clientX;
                let isDraggingSpeed = false;
                let isDraggingDuration = false;

                const handleMouseMove = (moveEvent) => {
                  if (!isDraggingSpeed && !isDraggingDuration) {
                    const deltaY = Math.abs(moveEvent.clientY - initialY);
                    const deltaX = Math.abs(moveEvent.clientX - initialX);
                    if (deltaY > deltaX) {
                      isDraggingSpeed = true;
                    } else {
                      isDraggingDuration = true;
                    }
                  }

                  if (isDraggingSpeed) {
                    handleSpeedDrag(moveEvent, segment.id, initialY);
                  } else if (isDraggingDuration) {
                    handleDurationDrag(moveEvent, segment.id, initialX);
                  }
                };

                const handleMouseUp = () => {
                  document.removeEventListener('mousemove', handleMouseMove);
                  document.removeEventListener('mouseup', handleMouseUp);
                };

                document.addEventListener('mousemove', handleMouseMove);
                document.addEventListener('mouseup', handleMouseUp);
              }}
            >
              <div className="text-xs text-white p-1">
                {segment.duration}ms, {(segment.speed * 0.06).toFixed(2)}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

function App() {
  const [track, setTrack] = useState([]);
  const [selectedMotor, setSelectedMotor] = useState(0);
  const [isDropdownOpen, setIsDropdownOpen] = useState(false);
  const dropdownRef = useRef(null);

  const addSegment = () => {
    setTrack([...track, { id: Date.now(), duration: 1000, speed: 50 }]);
  };

  const updateSegment = (id, field, value) => {
    setTrack(track.map(segment => 
      segment.id === id ? { ...segment, [field]: parseInt(value) } : segment
    ));
  };

  const removeSegment = (id) => {
    setTrack(track.filter(segment => segment.id !== id));
  };

  const formatPattern = () => {
    return {
      patterns: {
        motorId: selectedMotor,
        segments: track.map(({ duration, speed }) => ({
          duration,
          speed: parseFloat((speed * 0.06).toFixed(2)) // Clamp speed between 0 and 6 as a float
        }))
      }
    };
  };

  const sendPatternToServer = async () => {
    try {
      const pattern = formatPattern();
      const response = await axios.post('http://192.168.0.40:8080/pattern', pattern);
      console.log('Pattern sent successfully:', response.data);
      alert('Pattern sent successfully!');
    } catch (error) {
      console.error('Error sending pattern:', error);
      alert('Error sending pattern. Please try again.');
    }
  };

  useEffect(() => {
    function handleClickOutside(event) {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target)) {
        setIsDropdownOpen(false);
      }
    }

    document.addEventListener("mousedown", handleClickOutside);
    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
    };
  }, [dropdownRef]);

  return (
    <div className="App">
      <header className="App-header">
        <h1 className="text-2xl font-bold">Motor Pattern Editor</h1>
      </header>
      <main className="p-4">
        <div className="track-container border-2 border-gray-300 rounded-lg p-4 mb-4">
          <div className="flex items-center mb-4">
            <h2 className="text-xl font-semibold mr-2">Track</h2>
            <div className="relative inline-block text-left" ref={dropdownRef}>
              <div>
                <button 
                  type="button" 
                  className="inline-flex justify-center rounded-md border border-gray-300 shadow-sm px-4 py-2 bg-white text-sm font-medium text-gray-700 hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500" 
                  id="motor-menu" 
                  aria-haspopup="true" 
                  aria-expanded="true"
                  onClick={() => setIsDropdownOpen(!isDropdownOpen)}
                >
                  Motor {selectedMotor}
                  <svg className="-mr-1 ml-2 h-5 w-5" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
                    <path fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd" />
                  </svg>
                </button>
              </div>
              {isDropdownOpen && (
                <div className="origin-top-left absolute left-0 mt-2 w-56 rounded-md shadow-lg bg-white ring-1 ring-black ring-opacity-5">
                  <div className="py-1" role="menu" aria-orientation="vertical" aria-labelledby="motor-menu">
                    {[0, 1, 2, 3, 4, 5, 6].map((motorId) => (
                      <a
                        key={motorId}
                        href="#"
                        className={`${motorId === selectedMotor ? 'bg-gray-100 text-gray-900' : 'text-gray-700'} block px-4 py-2 text-sm`}
                        role="menuitem"
                        onClick={(e) => {
                          e.preventDefault();
                          setSelectedMotor(motorId);
                          setIsDropdownOpen(false);
                        }}
                      >
                        Motor {motorId}
                      </a>
                    ))}
                  </div>
                </div>
              )}
            </div>
          </div>
          <button 
            onClick={addSegment}
            className="mb-4 bg-blue-500 hover:bg-blue-700 text-white font-bold py-2 px-4 rounded"
          >
            Add Segment
          </button>
          <div className="track flex flex-wrap justify-center">
            {track.map(segment => (
              <div key={segment.id} className="segment bg-gray-100 rounded-lg p-4 m-2 w-64 relative">
                <button 
                  onClick={() => removeSegment(segment.id)}
                  className="absolute top-1 right-1 w-6 h-6 bg-red-500 hover:bg-red-700 text-white font-bold rounded-full flex items-center justify-center text-sm"
                >
                  ×
                </button>
                <div className="mb-2">
                  <label className="block text-sm font-medium text-gray-700">
                    Duration: {segment.duration}ms
                  </label>
                  <input
                    type="range"
                    min="0"
                    max="30000"
                    value={segment.duration}
                    onChange={(e) => updateSegment(segment.id, 'duration', e.target.value)}
                    className="w-full"
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700">
                    Speed: {segment.speed}%
                  </label>
                  <input
                    type="range"
                    min="0"
                    max="100"
                    value={segment.speed}
                    onChange={(e) => updateSegment(segment.id, 'speed', e.target.value)}
                    className="w-full"
                  />
                </div>
              </div>
            ))}
          </div>
        </div>
        <TrackVisualization track={track} updateSegment={updateSegment} />
        <div className="json-output border-2 border-gray-300 rounded-lg p-4">
          <h2 className="text-xl font-semibold mb-2">Pattern JSON</h2>
          <pre className="bg-gray-100 p-4 rounded overflow-x-auto">
            {JSON.stringify(formatPattern(), null, 2)}
          </pre>
          <button 
            onClick={sendPatternToServer}
            className="mt-4 bg-green-500 hover:bg-green-700 text-white font-bold py-2 px-4 rounded"
          >
            Send Pattern to Server
          </button>
        </div>
      </main>
    </div>
  );
}

export default App;
