import AdminDashboard from '../../../../../components/AdminDashboard'

export default function Page({ params }) {
  const { tenant } = params
  return <AdminDashboard tenant={tenant} />
}

