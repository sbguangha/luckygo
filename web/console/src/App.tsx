import type { ReactNode } from "react";
import { Navigate, Route, Routes } from "react-router-dom";
import { getToken, getRole } from "./api";
import Login from "./pages/Login";
import Admin from "./pages/Admin";
import Live from "./pages/Live";
import Join from "./pages/Join";
import Play from "./pages/Play";

function Guard({ role, children }: { role: string; children: ReactNode }) {
  if (!getToken()) return <Navigate to="/login" replace />;
  if (role && getRole() !== role) return <Navigate to={getRole() === "admin" ? "/admin" : "/login"} replace />;
  return <>{children}</>;
}

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route path="/admin" element={<Guard role="admin"><Admin /></Guard>} />
      <Route path="/live" element={<Live />} />
      <Route path="/join" element={<Join />} />
      <Route path="/p/:publicId" element={<Play />} />
      <Route path="*" element={<Navigate to="/login" replace />} />
    </Routes>
  );
}
