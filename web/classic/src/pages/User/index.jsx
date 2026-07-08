/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React from 'react';
import UsersTable from '../../components/table/users';
import UsersDepartmentPanel from '../../components/table/users/UsersDepartmentPanel';
import { Tabs, TabPane } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';

const User = () => {
  const { t } = useTranslation();
  const [activeTab, setActiveTab] = React.useState('users');

  return (
    <div className='mt-[60px] px-2'>
      <Tabs activeKey={activeTab} onChange={setActiveTab} className='mb-4'>
        <TabPane tab={t('用户管理')} itemKey='users'>
          <UsersTable />
        </TabPane>
        <TabPane tab={t('部门管理')} itemKey='departments'>
          <UsersDepartmentPanel />
        </TabPane>
      </Tabs>
    </div>
  );
};

export default User;
