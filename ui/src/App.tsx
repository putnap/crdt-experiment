import React, { useEffect, useRef, useState } from "react";
import { InitMessage, OperationMessage, ServerMessage } from "./types";
import { v4 as uuidv4 } from "uuid";
import { generateDiffOperations } from "./diff-utils"; // We'll show in next snippet

interface PresenceMap {
  [userID: string]: {
    userColor: string;
    cursorPos: number;
  };
}

const App: React.FC = () => {
  const [docId, setDocId] = useState("mydoc");
  const [text, setText] = useState("");
  const [presence, setPresence] = useState<PresenceMap>({});
  const wsRef = useRef<WebSocket | null>(null);

  // We'll store oldText in a ref for diff calculations
  const oldTextRef = useRef<string>(text);

  // We'll store our local userID/color for presence
  const [userID] = useState(() => `user-${uuidv4().slice(0, 8)}`);
  const [userColor] = useState(() => randomColor());

  // Cursor tracking
  const textAreaRef = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    const url = `ws://localhost:8080/ws/${docId}?userID=${userID}&color=${encodeURIComponent(userColor)}`;
    const socket = new WebSocket(url);

    socket.onopen = () => {
      console.log("WebSocket connected");
    };

    socket.onmessage = (e) => {
      const data: ServerMessage = JSON.parse(e.data);

      if (data.type === "init") {
        const initMsg = data as InitMessage;
        setText(initMsg.text);
        oldTextRef.current = initMsg.text;

        // Convert presence to a local map
        const presenceMap: PresenceMap = {};
        for (const userID in initMsg.presence) {
          const p = initMsg.presence[userID];
          presenceMap[userID] = {
            userColor: p.userColor,
            cursorPos: p.cursorPos,
          };
        }
        setPresence(presenceMap);
      } else {
        const op = data as OperationMessage;
        if (op.type === "insert") {
          applyInsert(op);
        } else if (op.type === "delete") {
          applyDelete(op);
        } else if (op.type === "cursor") {
          applyCursor(op);
        }
      }
    };

    socket.onclose = () => {
      console.log("WebSocket disconnected");
    };

    wsRef.current = socket;

    return () => {
      socket.close();
    };
  }, [docId, userColor, userID]);

  // Apply remote insert
  const applyInsert = (op: OperationMessage) => {
    setText((prev) => {
      if (op.position < 0 || op.position > prev.length) {
        return prev;
      }
      const newText =
        prev.slice(0, op.position) + (op.value || "") + prev.slice(op.position);
      oldTextRef.current = newText;
      return newText;
    });
  };

  const applyDelete = (op: OperationMessage) => {
    setText((prev) => {
      if (op.position < 0 || op.position >= prev.length) {
        return prev;
      }
      const deleteLen = op.value ? op.value.length : 1;
      const newText =
        prev.slice(0, op.position) + prev.slice(op.position + deleteLen);
      oldTextRef.current = newText;
      return newText;
    });
  };

  const applyCursor = (op: OperationMessage) => {
    setPresence((prev) => {
      return {
        ...prev,
        [op.source]: {
          userColor: op.userColor || "#000000",
          cursorPos: op.cursorPos ?? 0,
        },
      };
    });
  };

  const handleChange: React.ChangeEventHandler<HTMLTextAreaElement> = (e) => {
    const newVal = e.target.value;
    const oldVal = oldTextRef.current;

    if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) {
      setText(newVal);
      oldTextRef.current = newVal;
      return;
    }

    const ops = generateDiffOperations(oldVal, newVal, docId, userID);

    ops.forEach((op) => {
      wsRef.current?.send(JSON.stringify(op));
    });

    setText(newVal);
    oldTextRef.current = newVal;
  };

  // Track local cursor
  const handleCursorChange: React.FocusEventHandler<HTMLTextAreaElement> &
    React.FormEventHandler<HTMLTextAreaElement> = (e) => {
    if (!wsRef.current) return;
    const cursorPos = e.currentTarget.selectionStart;

    const op: OperationMessage = {
      type: "cursor",
      docId,
      position: 0,
      operationId: uuidv4(),
      source: userID,
      timestamp: Date.now(),
      cursorPos: cursorPos,
      userColor: userColor,
    };
    wsRef.current.send(JSON.stringify(op));
  };

  // Render presence cursors
  const renderCursors = () => {
    return Object.entries(presence).map(([uid, p]) => {
      if (uid === userID) return null; // skip our own
      const displayPos = Math.min(p.cursorPos, text.length);
      return (
        <div key={uid} style={{ color: p.userColor, marginTop: 4 }}>
          {uid} cursor at {displayPos}
        </div>
      );
    });
  };

  return (
    <div style={{ padding: 20 }}>
      <h2>Production-ish CRDT Editor</h2>
      <label>
        Document ID:
        <input
          type="text"
          value={docId}
          onChange={(e) => setDocId(e.target.value)}
          style={{ marginLeft: 8 }}
        />
      </label>

      <div style={{ marginTop: 10 }}>
        <textarea
          ref={textAreaRef}
          style={{ width: "100%", height: 200 }}
          value={text}
          onChange={handleChange}
          onSelect={handleCursorChange}
          onClick={handleCursorChange}
          onFocus={handleCursorChange}
        />
      </div>

      <h4>Remote Cursors:</h4>
      {renderCursors()}
    </div>
  );
};

function randomColor(): string {
  return "#" + Math.floor(Math.random() * 16777215).toString(16);
}

export default App;
