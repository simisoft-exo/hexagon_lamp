import React, { useState } from 'react';
import './App.css';
import MotorPatternEditor from './MotorPatternEditor';
import LightsPlaceholder from './LightsPlaceholder';

function App() {
  const [activeTab, setActiveTab] = useState('motors');

  return (
    <div className="App">
      <header className="App-header">
        <h1 className="text-2xl font-bold">Pattern Editor</h1>
      </header>
      <div className="tabs flex border-b">
        <button
          className={`py-2 px-4 ${activeTab === 'motors' ? 'bg-blue-500 text-white' : 'bg-gray-200'}`}
          onClick={() => setActiveTab('motors')}
        >
          Motors
        </button>
        <button
          className={`py-2 px-4 ${activeTab === 'lights' ? 'bg-blue-500 text-white' : 'bg-gray-200'}`}
          onClick={() => setActiveTab('lights')}
        >
          Lights
        </button>
      </div>
      <main className="p-4">
        {activeTab === 'motors' ? <MotorPatternEditor /> : <LightsPlaceholder />}
      </main>
    </div>
  );
}

export default App;
