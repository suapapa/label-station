import React, { useState, useEffect, useRef } from 'react';
import { QRCodeCanvas } from 'qrcode.react';
import './App.css';

function App() {
  const [info, setInfo] = useState(null);
  const [activeTab, setActiveTab] = useState('QR'); // QR, ADDR, IMG
  const [qrTab, setQrTab] = useState('WiFi'); // WiFi, contact, URL

  // Form Fields
  const [wifiSsid, setWifiSsid] = useState('');
  const [wifiPass, setWifiPass] = useState('');
  const [wifiType, setWifiType] = useState('WPA'); // WPA, WEP, nopass

  const [contactName, setContactName] = useState('');
  const [contactPhone, setContactPhone] = useState('');
  const [contactEmail, setContactEmail] = useState('');

  const [url, setUrl] = useState('');

  const [addrText, setAddrText] = useState('');
  const [imgFile, setImgFile] = useState(null);
  const [imgSrc, setImgSrc] = useState('');

  // Print Options
  const [selectedLabel, setSelectedLabel] = useState('62'); // Default to 62mm continuous
  const [cut, setCut] = useState(true);
  const [dither, setDither] = useState(false);
  const [red, setRed] = useState(false);
  const [rotate, setRotate] = useState('auto');

  const [isPrinting, setIsPrinting] = useState(false);
  const [message, setMessage] = useState({ text: '', type: '' });

  // UI States
  const [showSettings, setShowSettings] = useState(false);
  const [isDragging, setIsDragging] = useState(false);

  const addrCanvasRef = useRef(null);
  const imgCanvasRef = useRef(null);

  useEffect(() => {
    fetch('/api/v1/info')
      .then((res) => res.json())
      .then((data) => {
        setInfo(data);
        if (data.labels && data.labels.length > 0) {
          const has62 = data.labels.find((l) => l.Identifier === '62');
          if (has62) setSelectedLabel('62');
          else setSelectedLabel(data.labels[0].Identifier);
        }
      })
      .catch((err) => setMessage({ text: 'Failed to connect to backend', type: 'error' }));
  }, []);

  useEffect(() => {
    if (activeTab === 'ADDR' && addrCanvasRef.current) {
      const canvas = addrCanvasRef.current;
      const ctx = canvas.getContext('2d');
      const width = 554;
      const height = 300;
      canvas.width = width;
      canvas.height = height;

      ctx.fillStyle = '#ffffff';
      ctx.fillRect(0, 0, width, height);

      ctx.fillStyle = '#000000';
      ctx.font = 'bold 40px sans-serif';
      
      const lines = addrText.split('\n');
      lines.forEach((line, index) => {
        ctx.fillText(line, 40, 70 + index * 50);
      });
    }
  }, [addrText, activeTab]);

  useEffect(() => {
    if (activeTab === 'IMG' && imgSrc && imgCanvasRef.current) {
      const canvas = imgCanvasRef.current;
      const ctx = canvas.getContext('2d');
      const img = new Image();
      img.src = imgSrc;
      img.onload = () => {
        const width = 554;
        const scale = width / img.width;
        const height = img.height * scale;
        canvas.width = width;
        canvas.height = height;
        ctx.drawImage(img, 0, 0, width, height);
      };
    }
  }, [imgSrc, activeTab]);

  const handleFileChange = (e) => {
    const file = e.target.files[0];
    handleFile(file);
  };

  const handleFile = (file) => {
    if (file && file.type.startsWith('image/')) {
      setImgFile(file);
      const reader = new FileReader();
      reader.onload = (readEvent) => {
        setImgSrc(readEvent.target.result);
      };
      reader.readAsDataURL(file);
    }
  };

  const handleDragOver = (e) => {
    e.preventDefault();
    if (activeTab === 'IMG') setIsDragging(true);
  };

  const handleDragLeave = (e) => {
    e.preventDefault();
    setIsDragging(false);
  };

  const handleDrop = (e) => {
    e.preventDefault();
    setIsDragging(false);
    if (activeTab === 'IMG') {
      const file = e.dataTransfer.files[0];
      handleFile(file);
    }
  };

  const getQRValue = () => {
    if (qrTab === 'WiFi') {
      return `WIFI:T:${wifiType};S:${wifiSsid};P:${wifiPass};;`;
    } else if (qrTab === 'contact') {
      return `MECARD:N:${contactName};TEL:${contactPhone};EMAIL:${contactEmail};;`;
    } else {
      return url;
    }
  };

  const handlePrint = async () => {
    setIsPrinting(true);
    setMessage({ text: 'Printing in progress...', type: 'info' });

    let canvas = null;
    if (activeTab === 'QR') {
      canvas = document.querySelector('.qr-preview canvas');
    } else if (activeTab === 'ADDR') {
      canvas = addrCanvasRef.current;
    } else if (activeTab === 'IMG') {
      canvas = imgCanvasRef.current;
    }

    if (!canvas) {
      setMessage({ text: 'Nothing to print! Please preview first.', type: 'error' });
      setIsPrinting(false);
      return;
    }

    const dataUrl = canvas.toDataURL('image/png');

    const payload = {
      image: dataUrl,
      label: selectedLabel,
      options: {
        cut: cut,
        dither: dither,
        red: red,
        rotate: rotate,
        hq: true,
      },
    };

    try {
      const response = await fetch('/api/v1/print', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });

      if (response.ok) {
        setMessage({ text: 'Printed successfully!', type: 'success' });
      } else {
        const errText = await response.text();
        setMessage({ text: `Print failed: ${errText}`, type: 'error' });
      }
    } catch (error) {
      setMessage({ text: `Network error: ${error.message}`, type: 'error' });
    } finally {
      setIsPrinting(false);
    }
  };

  return (
    <div className="container">
      {message.text && (
        <div className={`notification ${message.type}`}>
          {message.type === 'error' && '🚨 '}
          {message.type === 'success' && '✅ '}
          {message.type === 'info' && '⏳ '}
          {message.text}
        </div>
      )}

      <header className="header">
        <div className="title-container">
          <h1>Label Station</h1>
          {info && <div className="subtitle">{info.model}</div>}
        </div>
        
        <button className="settings-toggle" onClick={() => setShowSettings(true)} title="Settings">
          ⚙️
        </button>
      </header>

      <div className="main-content">
        <div className="left-panel">
          <div className="tabs">
            {['QR', 'ADDR', 'IMG'].map((tab) => (
              <button
                key={tab}
                className={`tab-btn ${activeTab === tab ? 'active' : ''}`}
                onClick={() => setActiveTab(tab)}
              >
                {tab === 'QR' && '📱 QR Code'}
                {tab === 'ADDR' && '✍️ Address / Text'}
                {tab === 'IMG' && '🖼️ Image'}
              </button>
            ))}
          </div>

          <div className="form-area">
            {activeTab === 'QR' && (
              <div className="sub-tabs-container">
                <div className="sub-tabs">
                  {['WiFi', 'contact', 'URL'].map((sub) => (
                    <button
                      key={sub}
                      className={`sub-tab-btn ${qrTab === sub ? 'active' : ''}`}
                      onClick={() => setQrTab(sub)}
                    >
                      {sub.charAt(0).toUpperCase() + sub.slice(1)}
                    </button>
                  ))}
                </div>

                <div className="form-content">
                  {qrTab === 'WiFi' && (
                    <>
                      <div className="input-group">
                        <label>SSID</label>
                        <input
                          type="text"
                          placeholder="WiFi Name"
                          value={wifiSsid}
                          onChange={(e) => setWifiSsid(e.target.value)}
                        />
                      </div>
                      <div className="input-group">
                        <label>Password</label>
                        <input
                          type="password"
                          placeholder="Password"
                          value={wifiPass}
                          onChange={(e) => setWifiPass(e.target.value)}
                        />
                      </div>
                      <div className="input-group">
                        <label>Security</label>
                        <select value={wifiType} onChange={(e) => setWifiType(e.target.value)}>
                          <option value="WPA">WPA / WPA2</option>
                          <option value="WEP">WEP</option>
                          <option value="nopass">None</option>
                        </select>
                      </div>
                    </>
                  )}

                  {qrTab === 'contact' && (
                    <>
                      <div className="input-group">
                        <label>Name</label>
                        <input
                          type="text"
                          placeholder="Full Name"
                          value={contactName}
                          onChange={(e) => setContactName(e.target.value)}
                        />
                      </div>
                      <div className="input-group">
                        <label>Phone</label>
                        <input
                          type="text"
                          placeholder="+123456789"
                          value={contactPhone}
                          onChange={(e) => setContactPhone(e.target.value)}
                        />
                      </div>
                      <div className="input-group">
                        <label>Email</label>
                        <input
                          type="email"
                          placeholder="name@domain.com"
                          value={contactEmail}
                          onChange={(e) => setContactEmail(e.target.value)}
                        />
                      </div>
                    </>
                  )}

                  {qrTab === 'URL' && (
                    <div className="input-group">
                      <label>Target URL</label>
                      <input
                        type="url"
                        placeholder="https://example.com"
                        value={url}
                        onChange={(e) => setUrl(e.target.value)}
                      />
                    </div>
                  )}
                </div>
              </div>
            )}

            {activeTab === 'ADDR' && (
              <div className="form-content">
                <div className="input-group">
                  <label>Label Text</label>
                  <textarea
                    rows="6"
                    placeholder="Enter Address or Text..."
                    value={addrText}
                    onChange={(e) => setAddrText(e.target.value)}
                  />
                </div>
              </div>
            )}

            {activeTab === 'IMG' && (
              <div className="form-content">
                <div className="input-group">
                  <label>Upload Image</label>
                  <input type="file" accept="image/*" onChange={handleFileChange} />
                  <p className="help-text">JPG, PNG allowed. Will auto-scale.</p>
                </div>
              </div>
            )}
          </div>

          <div className="left-panel-footer">
            <button className="print-btn" onClick={handlePrint} disabled={isPrinting}>
              {isPrinting ? '⏳ PRINTING...' : '🖨️ PRINT'}
            </button>
          </div>
        </div>

        <div className="right-panel">
          <div className="preview-container">
            <div className="preview-title">Live Preview</div>
            <div 
              className={`preview-box ${activeTab} ${isDragging ? 'dragging' : ''}`}
              onDragOver={handleDragOver}
              onDragLeave={handleDragLeave}
              onDrop={handleDrop}
            >
              {activeTab === 'QR' && (
                <div className="qr-preview">
                  <QRCodeCanvas value={getQRValue() || ' '} size={500} />
                </div>
              )}
              {activeTab === 'ADDR' && (
                <canvas ref={addrCanvasRef} className="preview-canvas" />
              )}
              {activeTab === 'IMG' && (
                <div className="canvas-wrapper">
                  {!imgSrc && <div className="placeholder"><span> No Image Loaded</span><span>(Drop image here)</span></div>}
                  {isDragging && <div className="drop-overlay">Drop to Upload</div>}
                  <canvas ref={imgCanvasRef} className="preview-canvas" style={{ display: imgSrc ? 'block' : 'none' }} />
                </div>
              )}
            </div>
          </div>
        </div>
      </div>

      {showSettings && (
        <div className="settings-overlay" onClick={() => setShowSettings(false)}>
          <div className="settings-modal" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <h3>Print Settings</h3>
              <button className="close-btn" onClick={() => setShowSettings(false)}>×</button>
            </div>
            <div className="modal-body">
              <div className="print-settings">
                <div className="input-group">
                  <label>Label Size</label>
                  <select value={selectedLabel} onChange={(e) => setSelectedLabel(e.target.value)}>
                    {info && info.labels ? (
                      info.labels.map((l) => (
                        <option key={l.Identifier} value={l.Identifier}>
                          {l.Identifier} ({l.TapeSize[0]}x{l.TapeSize[1] || 'len'} mm)
                        </option>
                      ))
                    ) : (
                      <option value="62">62 mm</option>
                    )}
                  </select>
                </div>

                <div className="checkbox-group">
                  <label>
                    <input type="checkbox" checked={cut} onChange={(e) => setCut(e.target.checked)} />
                    Auto Cut
                  </label>
                  <label>
                    <input type="checkbox" checked={dither} onChange={(e) => setDither(e.target.checked)} />
                    Dither
                  </label>
                  <label>
                    <input type="checkbox" checked={red} onChange={(e) => setRed(e.target.checked)} />
                    Two-Color (Red/Black)
                  </label>
                </div>
              </div>
            </div>
            <div className="modal-footer">
              <button className="done-btn" onClick={() => setShowSettings(false)}>Done</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

export default App;
