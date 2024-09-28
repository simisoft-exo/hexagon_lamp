import React, { useState, useEffect } from 'react';
import axios from 'axios';
import TrackVisualization from './TrackVisualization';
import MotorAnimation from './MotorAnimation';

function MotorPatternEditor() {
  const [tracks, setTracks] = useState(Array(7).fill().map(() => []));
  const [selectedMotor, setSelectedMotor] = useState(0);
  const [selectedSegment, setSelectedSegment] = useState(null);
  const [isJsonBoxOpen, setIsJsonBoxOpen] = useState(false);
  const [motorAssignments, setMotorAssignments] = useState({});

  const addSegment = () => {
    setTracks(prevTracks => {
      const newTracks = [...prevTracks];
      newTracks[selectedMotor] = [...newTracks[selectedMotor], { id: Date.now(), duration: 1000, speed: 50 }];
      return newTracks;
    });
  };

  const updateSegment = (id, field, value) => {
    setTracks(prevTracks => {
      const newTracks = [...prevTracks];
      newTracks[selectedMotor] = newTracks[selectedMotor].map(segment => 
        segment.id === id ? { ...segment, [field]: parseInt(value) } : segment
      );
      return newTracks;
    });
  };

  const removeSegment = (id) => {
    setTracks(prevTracks => {
      const newTracks = [...prevTracks];
      newTracks[selectedMotor] = newTracks[selectedMotor].filter(segment => segment.id !== id);
      return newTracks;
    });
  };

  const formatPattern = () => {
    return {
      patterns: tracks.map((track, index) => ({
        motorId: index,
        segments: track.map(({ duration, speed }) => ({
          duration,
          speed: parseFloat((speed * 0.06).toFixed(2))
        }))
      })).filter(pattern => pattern.segments.length > 0)
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

  const handleMotorAssign = (motorIndex) => {
    setMotorAssignments(prev => {
      const newAssignments = { ...prev };
      if (newAssignments[motorIndex] === selectedMotor) {
        delete newAssignments[motorIndex];
      } else {
        newAssignments[motorIndex] = selectedMotor;
      }
      return newAssignments;
    });
  };

  const handleMotorSelect = (motorIndex) => {
    setSelectedMotor(motorIndex);
  };

  return (
    <div className="flex">
      {/* Left Pane: Segment Editor and Track Visualization */}
      <div className="w-1/2 pr-4">
        <div className="track-container border-2 border-gray-300 rounded-lg p-4 mb-4">
          <div className="flex items-center mb-4">
            <h2 className="text-xl font-semibold mr-2">Motor Track</h2>
            <div className="flex">
              {[0, 1, 2, 3, 4, 5, 6].map((motorId) => (
                <button
                  key={motorId}
                  className={`px-3 py-1 mr-1 rounded ${motorId === selectedMotor ? 'bg-blue-500 text-white' : 'bg-gray-200'}`}
                  onClick={() => setSelectedMotor(motorId)}
                >
                  Motor {motorId}
                </button>
              ))}
            </div>
          </div>
          <button 
            onClick={addSegment}
            className="mb-4 bg-blue-500 hover:bg-blue-700 text-white font-bold py-2 px-4 rounded"
          >
            Add Segment
          </button>
          <div className="track flex flex-col">
            {tracks[selectedMotor].map(segment => (
              <div 
                key={segment.id} 
                className={`segment bg-gray-100 rounded-lg p-4 mb-2 relative ${selectedSegment === segment.id ? 'border-2 border-blue-500' : ''}`}
                onClick={() => setSelectedSegment(segment.id)}
              >
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
        
        {/* Track Visualization */}
        <TrackVisualization 
          track={tracks[selectedMotor]} 
          updateSegment={updateSegment} 
          selectedSegment={selectedSegment} 
          setSelectedSegment={setSelectedSegment} 
        />
        
        <div className="json-output border-2 border-gray-300 rounded-lg p-4 mt-4">
          <div 
            className="flex justify-between items-center cursor-pointer"
            onClick={() => setIsJsonBoxOpen(!isJsonBoxOpen)}
          >
            <h2 className="text-xl font-semibold">Pattern JSON</h2>
            <svg 
              className={`w-6 h-6 transform transition-transform ${isJsonBoxOpen ? 'rotate-180' : ''}`} 
              fill="none" 
              stroke="currentColor" 
              viewBox="0 0 24 24" 
              xmlns="http://www.w3.org/2000/svg"
            >
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
            </svg>
          </div>
          {isJsonBoxOpen && (
            <pre className="bg-gray-100 p-4 rounded overflow-x-auto mt-2">
              {JSON.stringify(formatPattern(), null, 2)}
            </pre>
          )}
          <button 
            onClick={sendPatternToServer}
            className="mt-4 bg-green-500 hover:bg-green-700 text-white font-bold py-2 px-4 rounded"
          >
            Send Pattern to Server
          </button>
        </div>
      </div>

      {/* Right Pane: Motor Animation */}
      <div className="w-1/2 pl-4">
        <div className="border-2 border-gray-300 rounded-lg p-4 h-full">
          <h2 className="text-xl font-semibold mb-4">Motor Animation</h2>
          <MotorAnimation 
            tracks={tracks}
            selectedMotor={selectedMotor} 
            motorAssignments={motorAssignments}
            onMotorAssign={handleMotorAssign}
            onMotorSelect={handleMotorSelect}
          />
        </div>
      </div>
    </div>
  );
}

export default MotorPatternEditor;