"use client"
export default function AppointmentTable({ appointments, onEdit, onDelete }) {
  return (
    <div className="overflow-x-auto border rounded bg-white shadow">
      <table className="min-w-full text-sm">
        <thead className="bg-gray-100">
          <tr>
            <Th>ID</Th>
            <Th>Patient</Th>
            <Th>Doctor</Th>
            <Th>Date</Th>
            <Th>Time</Th>
            <Th>Reason</Th>
            <Th>Status</Th>
            <Th>Actions</Th>
          </tr>
        </thead>
        <tbody>
          {appointments.map((a) => (
            <tr key={a.id} className="border-t">
              <Td>{a.id}</Td>
              <Td>{a.patient_name}</Td>
              <Td>{a.doctor}</Td>
              <Td>{a.date}</Td>
              <Td>{a.time}</Td>
              <Td>{a.reason}</Td>
              <Td>{a.status}</Td>
              <Td>
                <div className="flex gap-2">
                  <button className="px-2 py-1 rounded bg-yellow-500 text-white" onClick={() => onEdit(a)}>Edit</button>
                  <button className="px-2 py-1 rounded bg-red-600 text-white" onClick={() => onDelete(a)}>Delete</button>
                </div>
              </Td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function Th({ children }) { return <th className="text-left p-2 whitespace-nowrap">{children}</th> }
function Td({ children }) { return <td className="p-2 align-top whitespace-nowrap">{children}</td> }
