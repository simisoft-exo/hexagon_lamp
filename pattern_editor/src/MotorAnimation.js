import React, { useEffect, useRef, useState } from 'react';

const MotorAnimation = ({ tracks, selectedMotor, motorAssignments, onMotorAssign }) => {
  const canvasRef = useRef(null);
  const [hoveredMotor, setHoveredMotor] = useState(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    const ctx = canvas.getContext('2d');
    const centerX = canvas.width / 2;
    const centerY = canvas.height / 2;
    const radius = 20;
    const outerRadius = 100;

    const drawMotors = () => {
      ctx.clearRect(0, 0, canvas.width, canvas.height);

      // Draw center motor
      drawMotor(ctx, centerX, centerY, radius, 0);

      // Draw outer motors
      for (let i = 0; i < 6; i++) {
        const angle = (i * Math.PI) / 3;
        const x = centerX + outerRadius * Math.cos(angle);
        const y = centerY + outerRadius * Math.sin(angle);
        drawMotor(ctx, x, y, radius, i + 1);
      }
    };

    const drawMotor = (ctx, x, y, radius, motorIndex) => {
      ctx.beginPath();
      ctx.arc(x, y, radius, 0, 2 * Math.PI);
      ctx.fillStyle = getMotorColor(motorIndex);
      ctx.fill();
      ctx.strokeStyle = 'black';
      ctx.lineWidth = 2;
      ctx.stroke();

      // Draw motor number
      ctx.fillStyle = 'white';
      ctx.font = '16px Arial';
      ctx.textAlign = 'center';
      ctx.textBaseline = 'middle';
      ctx.fillText(motorAssignments[motorIndex] !== undefined ? motorAssignments[motorIndex] : '-', x, y);
    };

    const getMotorColor = (motorIndex) => {
      if (hoveredMotor === motorIndex) return 'lightblue';
      if (motorAssignments[motorIndex] !== undefined) return 'green';
      return 'gray';
    };

    drawMotors();

    // Animation loop
    let currentSegmentIndices = tracks.map(() => 0);
    let currentTimes = tracks.map(() => 0);

    const animate = () => {
      ctx.clearRect(0, 0, canvas.width, canvas.height);
      drawMotors();

      tracks.forEach((track, motorIndex) => {
        if (track.length === 0) return;

        const currentSegment = track[currentSegmentIndices[motorIndex]];
        currentTimes[motorIndex] += 16; // Assuming 60 FPS

        if (currentTimes[motorIndex] >= currentSegment.duration) {
          currentTimes[motorIndex] = 0;
          currentSegmentIndices[motorIndex] = (currentSegmentIndices[motorIndex] + 1) % track.length;
        }

        const progress = currentTimes[motorIndex] / currentSegment.duration;
        const speed = currentSegment.speed * 0.06; // Convert to 0-6 range

        // Update motor position based on speed and progress
        const angle = progress * speed * Math.PI * 2;
        const x = centerX + outerRadius * Math.cos(angle);
        const y = centerY + outerRadius * Math.sin(angle);

        // Draw animation indicator for this motor
        ctx.beginPath();
        ctx.arc(x, y, 5, 0, 2 * Math.PI);
        ctx.fillStyle = motorIndex === selectedMotor ? 'blue' : 'rgba(0, 0, 255, 0.3)';
        ctx.fill();
      });

      requestAnimationFrame(animate);
    };

    animate();
  }, [tracks, selectedMotor, motorAssignments, hoveredMotor]);

  const handleCanvasClick = (event) => {
    const canvas = canvasRef.current;
    const rect = canvas.getBoundingClientRect();
    const x = event.clientX - rect.left;
    const y = event.clientY - rect.top;
    const clickedMotor = getClickedMotor(x, y);
    if (clickedMotor !== null) {
      onMotorAssign(clickedMotor);
    }
  };

  const handleCanvasMouseMove = (event) => {
    const canvas = canvasRef.current;
    const rect = canvas.getBoundingClientRect();
    const x = event.clientX - rect.left;
    const y = event.clientY - rect.top;
    const hoveredMotorIndex = getClickedMotor(x, y);
    setHoveredMotor(hoveredMotorIndex);
  };

  const getClickedMotor = (x, y) => {
    const centerX = canvasRef.current.width / 2;
    const centerY = canvasRef.current.height / 2;
    const radius = 20;
    const outerRadius = 100;

    // Check center motor
    if (Math.sqrt((x - centerX) ** 2 + (y - centerY) ** 2) <= radius) {
      return 0;
    }

    // Check outer motors
    for (let i = 0; i < 6; i++) {
      const angle = (i * Math.PI) / 3;
      const motorX = centerX + outerRadius * Math.cos(angle);
      const motorY = centerY + outerRadius * Math.sin(angle);
      if (Math.sqrt((x - motorX) ** 2 + (y - motorY) ** 2) <= radius) {
        return i + 1;
      }
    }

    return null;
  };

  return (
    <canvas 
      ref={canvasRef} 
      width="300" 
      height="300" 
      onClick={handleCanvasClick}
      onMouseMove={handleCanvasMouseMove}
      onMouseLeave={() => setHoveredMotor(null)}
      style={{ cursor: 'pointer' }}
    />
  );
};

export default MotorAnimation;