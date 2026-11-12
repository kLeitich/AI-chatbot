import AdminDashboard from '../../../../../components/AdminDashboard'
import { PermissionProvider } from '../../../../../context/PermissionContext'

export default function Page({ params }) {
  const { tenant } = params
  return (
    <PermissionProvider tenant={tenant}>
      <AdminDashboard tenant={tenant} />
    </PermissionProvider>
  )
}
